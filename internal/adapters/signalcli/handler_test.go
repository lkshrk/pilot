package signalcli

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	signal "github.com/lkshrk/pilot-signal-adapter/signalcli"
)

const testGroupRaw = "Km7Q/DSD6h3XlGjsZ5BSZ+nKbdukhEe/2rYzaJYwmrw="

type fakeReceiver struct {
	frames []signal.Frame
	err    error
}

func (f *fakeReceiver) Run(_ context.Context, onFrame func(signal.Frame)) error {
	for _, fr := range f.frames {
		onFrame(fr)
	}
	return f.err
}

func frame(t *testing.T, raw string) signal.Frame {
	t.Helper()
	f, err := signal.ParseFrame([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return f
}

func groupMessage(sender, text string) string {
	return `{"envelope":{"sourceUuid":"` + sender + `","sourceName":"Tester","timestamp":1786854659376,
	  "dataMessage":{"message":"` + text + `","groupInfo":{"groupId":"` + testGroupRaw + `","groupName":"g"}}},
	  "account":"+490000000001"}`
}

func groupVote(sender string, option int) string {
	return `{"envelope":{"sourceUuid":"` + sender + `","sourceName":"Tester","timestamp":1786854659376,
	  "dataMessage":{"message":null,"pollVote":{"authorUuid":"bot-uuid","targetSentTimestamp":42,
	  "optionIndexes":[` + itoa(option) + `],"voteCount":1},
	  "groupInfo":{"groupId":"` + testGroupRaw + `","groupName":"g"}}},"account":"+490000000001"}`
}

func itoa(i int) string { return string(rune('0' + i)) }

func TestVotesAreRoutedToTheVoteSink(t *testing.T) {
	h := NewHandler(&HandlerConfig{Groups: []string{testGroupRaw}}, nil)
	var got []signal.Vote
	h.SetVoteSink(func(_ context.Context, v signal.Vote) { got = append(got, v) })
	h.SetReceiver(&fakeReceiver{frames: []signal.Frame{frame(t, groupVote("human", 0))}})

	if err := h.StartListening(context.Background()); err != nil {
		t.Fatalf("StartListening: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("votes = %d, want 1", len(got))
	}
	if got[0].VoterID != "human" {
		t.Errorf("VoterID = %q, want the envelope sender not the poll author", got[0].VoterID)
	}
	if got[0].PollTimestamp != 42 {
		t.Errorf("PollTimestamp = %d", got[0].PollTimestamp)
	}
}

// A message from a group pilot does not serve must never reach dispatch —
// group membership is what grants approval authority.
func TestFramesFromOtherGroupsAreDropped(t *testing.T) {
	h := NewHandler(&HandlerConfig{Groups: []string{"some-other-group"}}, nil)
	var votes int
	h.SetVoteSink(func(context.Context, signal.Vote) { votes++ })
	h.SetReceiver(&fakeReceiver{frames: []signal.Frame{
		frame(t, groupVote("human", 0)),
		frame(t, groupMessage("human", "deploy")),
	}})

	if err := h.StartListening(context.Background()); err != nil {
		t.Fatalf("StartListening: %v", err)
	}
	if votes != 0 {
		t.Errorf("votes = %d, want 0 from a non-allowlisted group", votes)
	}
}

// The allowlist must accept the send-side encoding too, since config is often
// copied from GET /v1/groups.
func TestAllowlistAcceptsSendSideGroupEncoding(t *testing.T) {
	sendForm := "group." + base64.StdEncoding.EncodeToString([]byte(testGroupRaw))
	h := NewHandler(&HandlerConfig{Groups: []string{sendForm}}, nil)
	var votes int
	h.SetVoteSink(func(context.Context, signal.Vote) { votes++ })
	h.SetReceiver(&fakeReceiver{frames: []signal.Frame{frame(t, groupVote("human", 0))}})

	if err := h.StartListening(context.Background()); err != nil {
		t.Fatalf("StartListening: %v", err)
	}
	if votes != 1 {
		t.Errorf("votes = %d — send-form config did not match received frames", votes)
	}
}

// Pilot's own traffic must not round-trip into its command path.
func TestOwnMessagesAreIgnored(t *testing.T) {
	h := NewHandler(&HandlerConfig{Groups: []string{testGroupRaw}}, nil)
	h.SetSelfUUID("pilot-uuid")
	var votes int
	h.SetVoteSink(func(context.Context, signal.Vote) { votes++ })
	h.SetReceiver(&fakeReceiver{frames: []signal.Frame{frame(t, groupVote("pilot-uuid", 0))}})

	if err := h.StartListening(context.Background()); err != nil {
		t.Fatalf("StartListening: %v", err)
	}
	if votes != 0 {
		t.Error("pilot's own frame was processed")
	}
}

func TestStartListeningRequiresAReceiver(t *testing.T) {
	h := NewHandler(&HandlerConfig{Groups: []string{testGroupRaw}}, nil)
	if err := h.StartListening(context.Background()); err == nil {
		t.Fatal("started with no receiver")
	}
}

func TestTransportErrorIsPropagated(t *testing.T) {
	want := errors.New("fatal close")
	h := NewHandler(&HandlerConfig{Groups: []string{testGroupRaw}}, nil)
	h.SetReceiver(&fakeReceiver{err: want})

	if err := h.StartListening(context.Background()); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}
