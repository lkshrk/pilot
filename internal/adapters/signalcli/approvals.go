package signalcli

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	signal "github.com/lkshrk/pilot-signal-adapter/signalcli"
	"github.com/qf-studio/pilot/internal/comms"
)

// callback action IDs understood by comms.Handler's confirmation path.
const (
	actionExecute = "execute"
	actionCancel  = "cancel"
)

// projectSource resolves the project a poll's stored path belongs to. Polls
// carry the active project path, while approvers are configured by name.
type projectSource interface {
	GetProjectByPath(path string) *comms.ProjectInfo
}

// pollRegistry reports whether a poll is one pilot is still waiting on, and
// which project it decides.
type pollRegistry interface {
	PendingProject(ref string) (string, bool)
	SendText(ctx context.Context, contextID, threadID, text string) error
}

// dispatcher is the confirmation path a vote is translated into. Narrowed to
// the single method used so the router can be tested without a live handler.
type dispatcher interface {
	HandleMessage(ctx context.Context, msg *comms.IncomingMessage)
}

// ApprovalRouter turns poll votes into confirmations.
//
// Votes are the only way an approval arrives over Signal, and comms.Handler
// already owns confirmation semantics, so this translates a vote into the
// callback that path expects rather than deciding anything itself.
type ApprovalRouter struct {
	comms      dispatcher
	polls      pollRegistry
	projects   projectSource
	approvers  []string
	perProject map[string][]string
	selfUUID   string
	log        *slog.Logger
}

// ApprovalConfig configures an [ApprovalRouter].
type ApprovalConfig struct {
	Approvers        []string
	ProjectApprovers map[string][]string
	SelfUUID         string
	Projects         projectSource
	Logger           *slog.Logger
}

// NewApprovalRouter builds a router over the shared comms handler.
func NewApprovalRouter(c dispatcher, polls pollRegistry, cfg *ApprovalConfig) *ApprovalRouter {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &ApprovalRouter{
		comms:      c,
		polls:      polls,
		projects:   cfg.Projects,
		approvers:  cfg.Approvers,
		perProject: cfg.ProjectApprovers,
		selfUUID:   cfg.SelfUUID,
		log:        log,
	}
}

// Sink returns the vote handler to register with [Handler.SetVoteSink].
func (r *ApprovalRouter) Sink() func(context.Context, signal.Vote) {
	return r.handle
}

func (r *ApprovalRouter) handle(ctx context.Context, vote signal.Vote) {
	if r.comms == nil {
		return
	}

	// A poll pilot did not author is a group member's own poll sharing the
	// stream. Checked before the approver rules so an unrelated poll never
	// draws a refusal into the group.
	if r.selfUUID != "" && vote.PollAuthorID != r.selfUUID {
		return
	}

	ref := strconv.FormatInt(vote.PollTimestamp, 10)
	project, pending := r.polls.PendingProject(ref)
	if !pending {
		return
	}

	if !r.mayApprove(project, vote.VoterID) {
		r.log.Warn("unauthorized signal approval vote",
			"poll", ref, "voter", vote.VoterID, "project", project)
		if err := r.polls.SendText(ctx, vote.GroupID, "", "Vote ignored — not an approver."); err != nil {
			r.log.Warn("failed to report refused approval vote", "error", err)
		}
		return
	}

	action := actionCancel
	if len(vote.OptionIndexes) > 0 && vote.OptionIndexes[0] == approveIndex {
		action = actionExecute
	}

	r.log.Info("signal approval vote accepted",
		"poll", ref, "voter", vote.VoterID, "project", project, "action", action)

	r.comms.HandleMessage(ctx, &comms.IncomingMessage{
		ContextID:  vote.GroupID,
		SenderID:   vote.VoterID,
		Platform:   platform,
		IsCallback: true,
		CallbackID: ref,
		ActionID:   action,
		Timestamp:  time.UnixMilli(vote.ReceivedAt),
	})
}

// mayApprove applies the per-project list first: a project named in the config
// is decided only by its own approvers, so listing it with an empty list
// approves nothing. Falling through an absent entry to the global list, and an
// empty global list to group membership, keeps the documented default.
func (r *ApprovalRouter) mayApprove(project, voter string) bool {
	if name, ok := r.projectName(project); ok {
		if allowed, configured := r.perProject[name]; configured {
			return contains(allowed, voter)
		}
	}
	if len(r.approvers) == 0 {
		return true
	}
	return contains(r.approvers, voter)
}

// projectName maps the poll's stored project path to its configured name.
func (r *ApprovalRouter) projectName(path string) (string, bool) {
	if path == "" || r.projects == nil {
		return "", false
	}
	proj := r.projects.GetProjectByPath(path)
	if proj == nil || proj.Name == "" {
		return "", false
	}
	return proj.Name, true
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
