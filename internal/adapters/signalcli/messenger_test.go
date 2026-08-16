package signalcli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/qf-studio/pilot/internal/comms"
)

// The whole point of this package is to satisfy pilot's interface; a signature
// drift should fail the build, not a review.
var _ comms.Messenger = (*Messenger)(nil)

type fakeSender struct {
	sentTo    []string
	sentText  []string
	chunked   []string
	polls     []string
	pollTo    []string
	closed    []int64
	closedTo  []string
	nextTS    int64
	failClose error
	failSend  error
}

func (f *fakeSender) SendText(_ context.Context, recipient, text string) (int64, error) {
	if f.failSend != nil {
		return 0, f.failSend
	}
	f.sentTo = append(f.sentTo, recipient)
	f.sentText = append(f.sentText, text)
	f.nextTS++
	return f.nextTS, nil
}

func (f *fakeSender) SendChunked(_ context.Context, recipient, text string) ([]int64, error) {
	f.sentTo = append(f.sentTo, recipient)
	f.chunked = append(f.chunked, text)
	return []int64{1}, nil
}

func (f *fakeSender) CreatePoll(_ context.Context, recipient, question string, _ []string, _ bool) (int64, error) {
	f.pollTo = append(f.pollTo, recipient)
	f.polls = append(f.polls, question)
	f.nextTS++
	return f.nextTS, nil
}

func (f *fakeSender) ClosePoll(_ context.Context, recipient string, ts int64) error {
	if f.failClose != nil {
		return f.failClose
	}
	f.closedTo = append(f.closedTo, recipient)
	f.closed = append(f.closed, ts)
	return nil
}

func (f *fakeSender) MaxMessageLength() int { return 2000 }

const rawGroupID = "Km7Q/DSD6h3XlGjsZ5BSZ+nKbdukhEe/2rYzaJYwmrw="

// Pilot's contextID may carry the received group encoding, which the send
// endpoints do not accept. Converting here is what stops replies going nowhere.
func TestContextIDIsConvertedToASendRecipient(t *testing.T) {
	f := &fakeSender{}
	m := NewMessenger(f)

	if err := m.SendText(context.Background(), rawGroupID, "", "hi"); err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if len(f.sentTo) != 1 {
		t.Fatalf("sends = %d", len(f.sentTo))
	}
	if f.sentTo[0] == rawGroupID {
		t.Error("received-form group ID passed through unconverted")
	}
	if !strings.HasPrefix(f.sentTo[0], "group.") {
		t.Errorf("recipient = %q, want the send form", f.sentTo[0])
	}
}

// Closing the poll is what makes an approval final, so the ref must resolve back
// to the group the poll was posted in.
func TestAcknowledgeCallbackClosesTheConfirmationPoll(t *testing.T) {
	f := &fakeSender{}
	m := NewMessenger(f)

	ref, err := m.SendConfirmation(context.Background(), rawGroupID, "", "HCL-1", "do a thing", "proj")
	if err != nil {
		t.Fatalf("SendConfirmation: %v", err)
	}
	if len(f.polls) != 1 {
		t.Fatalf("polls = %d, want the confirmation posted as a poll", len(f.polls))
	}
	if err := m.AcknowledgeCallback(context.Background(), ref); err != nil {
		t.Fatalf("AcknowledgeCallback: %v", err)
	}
	if len(f.closed) != 1 {
		t.Fatalf("closed = %v, want the poll closed", f.closed)
	}
	if f.closedTo[0] != f.pollTo[0] {
		t.Errorf("closed in %q but posted to %q", f.closedTo[0], f.pollTo[0])
	}
}

// A failed close means the approval is still mutable, so the ref must survive
// for a retry rather than being dropped.
func TestFailedCloseKeepsTheRefForRetry(t *testing.T) {
	f := &fakeSender{failClose: errors.New("boom")}
	m := NewMessenger(f)

	ref, err := m.SendConfirmation(context.Background(), rawGroupID, "", "HCL-1", "d", "p")
	if err != nil {
		t.Fatalf("SendConfirmation: %v", err)
	}
	if err := m.AcknowledgeCallback(context.Background(), ref); err == nil {
		t.Fatal("failed close reported as success")
	}

	f.failClose = nil
	if err := m.AcknowledgeCallback(context.Background(), ref); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if len(f.closed) != 1 {
		t.Errorf("closed = %v, want the retry to succeed", f.closed)
	}
}

// Pilot acknowledges callbacks that never came from a poll; failing those would
// turn a no-op into a spurious error.
func TestAcknowledgeUnknownCallbackIsANoOp(t *testing.T) {
	f := &fakeSender{}
	m := NewMessenger(f)

	if err := m.AcknowledgeCallback(context.Background(), "not-a-ref"); err != nil {
		t.Errorf("unknown callback errored: %v", err)
	}
	if len(f.closed) != 0 {
		t.Error("unknown callback closed something")
	}
}

func TestSendChunkedAppliesPrefix(t *testing.T) {
	f := &fakeSender{}
	m := NewMessenger(f)

	if err := m.SendChunked(context.Background(), rawGroupID, "", "body", "PREFIX"); err != nil {
		t.Fatalf("SendChunked: %v", err)
	}
	if len(f.chunked) != 1 || !strings.HasPrefix(f.chunked[0], "PREFIX") {
		t.Errorf("chunked = %q", f.chunked)
	}
}

func TestFormatting(t *testing.T) {
	if got := confirmationQuestion("HCL-1", "line one\nline two", "proj"); strings.Contains(got, "\n") {
		t.Errorf("poll question contains a newline: %q", got)
	}
	long := strings.Repeat("x", 500)
	if got := confirmationQuestion("HCL-1", long, "proj"); len([]rune(got)) > 200 {
		t.Errorf("poll question not trimmed: %d runes", len([]rune(got)))
	}
	if got := resultText("HCL-1", false, "", ""); !strings.Contains(got, "failed") {
		t.Errorf("resultText = %q", got)
	}
	if got := progressText("HCL-1", "build", 50, "detail"); !strings.Contains(got, "50%") {
		t.Errorf("progressText = %q", got)
	}
}
