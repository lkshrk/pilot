package signalcli

import (
	"context"
	"testing"

	signal "github.com/lkshrk/pilot-signal-adapter/signalcli"
	"github.com/qf-studio/pilot/internal/comms"
)

// Values captured from a real vote envelope on the deployed instance.
const (
	pilotUUID  = "2f9b411e-7990-43ea-8293-86d0e6c7702e"
	voterUUID  = "3e04424c-c6d6-409f-8fb3-ff697bb217c4"
	groupID    = "Km7Q/DSD6h3XlGjsZ5BSZ+nKbdukhEe/2rYzaJYwmrw="
	pollTS     = int64(1786877410526)
	omniPath   = "/home/pilot/repos/omni"
	otherPath  = "/home/pilot/repos/other"
	pollRefStr = "1786877410526"
)

type fakeDispatcher struct{ got []*comms.IncomingMessage }

func (f *fakeDispatcher) HandleMessage(_ context.Context, msg *comms.IncomingMessage) {
	f.got = append(f.got, msg)
}

type fakePolls struct {
	project string
	known   bool
	sent    []string
}

func (f *fakePolls) PendingProject(string) (string, bool) { return f.project, f.known }

func (f *fakePolls) SendText(_ context.Context, _, _, text string) error {
	f.sent = append(f.sent, text)
	return nil
}

type fakeProjects map[string]string // path -> name

func (f fakeProjects) GetProjectByPath(path string) *comms.ProjectInfo {
	name, ok := f[path]
	if !ok {
		return nil
	}
	return &comms.ProjectInfo{Name: name, Path: path}
}

func voteOn(option int) signal.Vote {
	return signal.Vote{
		PollTimestamp: pollTS,
		PollAuthorID:  pilotUUID,
		OptionIndexes: []int{option},
		GroupID:       groupID,
		VoterID:       voterUUID,
		Revision:      1,
	}
}

func newRouter(t *testing.T, polls *fakePolls, cfg *ApprovalConfig) (*ApprovalRouter, *fakeDispatcher) {
	t.Helper()
	disp := &fakeDispatcher{}
	if cfg.Projects == nil {
		cfg.Projects = fakeProjects{omniPath: "omni", otherPath: "other"}
	}
	if cfg.SelfUUID == "" {
		cfg.SelfUUID = pilotUUID
	}
	return NewApprovalRouter(disp, polls, cfg), disp
}

func TestApproveDispatchesExecute(t *testing.T) {
	polls := &fakePolls{project: omniPath, known: true}
	r, disp := newRouter(t, polls, &ApprovalConfig{})

	r.handle(context.Background(), voteOn(approveIndex))

	if len(disp.got) != 1 {
		t.Fatalf("dispatched %d messages, want 1", len(disp.got))
	}
	msg := disp.got[0]
	if msg.ActionID != actionExecute {
		t.Errorf("ActionID = %q, want %q", msg.ActionID, actionExecute)
	}
	if !msg.IsCallback {
		t.Error("IsCallback = false, want true")
	}
	if msg.CallbackID != pollRefStr {
		t.Errorf("CallbackID = %q, want %q", msg.CallbackID, pollRefStr)
	}
	if msg.ContextID != groupID {
		t.Errorf("ContextID = %q, want the group ID", msg.ContextID)
	}
	// Attribution must come from the envelope, never the poll's author fields:
	// the author of an approval poll is pilot itself.
	if msg.SenderID != voterUUID {
		t.Errorf("SenderID = %q, want the voter %q", msg.SenderID, voterUUID)
	}
}

func TestRejectDispatchesCancel(t *testing.T) {
	polls := &fakePolls{project: omniPath, known: true}
	r, disp := newRouter(t, polls, &ApprovalConfig{})

	r.handle(context.Background(), voteOn(rejectIndex))

	if len(disp.got) != 1 {
		t.Fatalf("dispatched %d messages, want 1", len(disp.got))
	}
	if disp.got[0].ActionID != actionCancel {
		t.Errorf("ActionID = %q, want %q", disp.got[0].ActionID, actionCancel)
	}
}

func TestVoteOnForeignPollIgnored(t *testing.T) {
	polls := &fakePolls{project: omniPath, known: true}
	r, disp := newRouter(t, polls, &ApprovalConfig{})

	vote := voteOn(approveIndex)
	vote.PollAuthorID = "someone-else"
	r.handle(context.Background(), vote)

	if len(disp.got) != 0 {
		t.Errorf("dispatched %d messages for a poll pilot did not author, want 0", len(disp.got))
	}
	if len(polls.sent) != 0 {
		t.Errorf("replied %q to a foreign poll; must stay silent", polls.sent)
	}
}

func TestVoteOnUnknownRefIgnored(t *testing.T) {
	polls := &fakePolls{known: false}
	r, disp := newRouter(t, polls, &ApprovalConfig{})

	r.handle(context.Background(), voteOn(approveIndex))

	if len(disp.got) != 0 {
		t.Errorf("dispatched %d messages for an untracked poll, want 0", len(disp.got))
	}
	if len(polls.sent) != 0 {
		t.Errorf("replied %q to an untracked poll; must stay silent", polls.sent)
	}
}

func TestNonApproverRefusedAndTold(t *testing.T) {
	polls := &fakePolls{project: omniPath, known: true}
	r, disp := newRouter(t, polls, &ApprovalConfig{Approvers: []string{"someone-else"}})

	r.handle(context.Background(), voteOn(approveIndex))

	if len(disp.got) != 0 {
		t.Errorf("dispatched %d messages for a non-approver, want 0", len(disp.got))
	}
	if len(polls.sent) != 1 {
		t.Fatalf("sent %d replies, want 1 refusal", len(polls.sent))
	}
}

func TestMayApprove(t *testing.T) {
	tests := []struct {
		name      string
		approvers []string
		perProj   map[string][]string
		project   string
		voter     string
		want      bool
	}{
		{
			name:    "no lists configured falls back to group membership",
			project: omniPath, voter: voterUUID, want: true,
		},
		{
			name:      "global list admits a listed voter",
			approvers: []string{voterUUID},
			project:   omniPath, voter: voterUUID, want: true,
		},
		{
			name:      "global list refuses an unlisted voter",
			approvers: []string{"other-uuid"},
			project:   omniPath, voter: voterUUID, want: false,
		},
		{
			name:      "project list overrides the global list",
			approvers: []string{"other-uuid"},
			perProj:   map[string][]string{"omni": {voterUUID}},
			project:   omniPath, voter: voterUUID, want: true,
		},
		{
			name:      "project list narrows a voter the global list admits",
			approvers: []string{voterUUID},
			perProj:   map[string][]string{"omni": {"other-uuid"}},
			project:   omniPath, voter: voterUUID, want: false,
		},
		{
			name:      "project entry present but empty approves nothing",
			approvers: []string{voterUUID},
			perProj:   map[string][]string{"omni": {}},
			project:   omniPath, voter: voterUUID, want: false,
		},
		{
			name:      "project absent from the map falls back to global",
			approvers: []string{voterUUID},
			perProj:   map[string][]string{"other": {"other-uuid"}},
			project:   omniPath, voter: voterUUID, want: true,
		},
		{
			name:      "unresolvable project path falls back to global",
			approvers: []string{voterUUID},
			perProj:   map[string][]string{"omni": {"other-uuid"}},
			project:   "/unknown/path", voter: voterUUID, want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := newRouter(t, &fakePolls{}, &ApprovalConfig{
				Approvers:        tt.approvers,
				ProjectApprovers: tt.perProj,
			})
			if got := r.mayApprove(tt.project, tt.voter); got != tt.want {
				t.Errorf("mayApprove(%q, %q) = %v, want %v", tt.project, tt.voter, got, tt.want)
			}
		})
	}
}
