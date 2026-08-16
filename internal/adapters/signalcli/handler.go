package signalcli

import (
	"context"
	"errors"
	"log/slog"
	"time"

	signal "github.com/lkshrk/pilot-signal-adapter/signalcli"
	"github.com/qf-studio/pilot/internal/comms"
)

// platform identifies this adapter in comms.IncomingMessage.
const platform = "signal"

// receiver streams frames until the context ends, reconnecting internally.
type receiver interface {
	Run(ctx context.Context, onFrame func(signal.Frame)) error
}

// HandlerConfig configures a [Handler].
type HandlerConfig struct {
	// Groups are the only groups pilot will act on. Membership of an
	// allowlisted group is what grants approval authority, so this is the
	// authorization boundary and not merely noise filtering.
	Groups []string
	Logger *slog.Logger
}

// Handler feeds inbound Signal traffic into pilot's shared command dispatch.
//
// It deliberately owns no dispatch logic of its own: internal/comms already
// implements command routing, active-project state, rate limiting and intake
// for every chat adapter, and duplicating any of that here would put it on the
// fork's rebase surface for no gain.
type Handler struct {
	rx         receiver
	allow      *signal.GroupAllowlist
	log        *slog.Logger
	comms      *comms.Handler
	onVote     func(context.Context, signal.Vote)
	selfUUID   string
	groupCount int
}

// NewHandler builds a Handler. commsHandler may be nil, in which case inbound
// messages are logged and dropped — useful while wiring, and harmless.
func NewHandler(cfg *HandlerConfig, commsHandler *comms.Handler) *Handler {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Handler{
		allow:      signal.NewGroupAllowlist(cfg.Groups...),
		log:        log,
		comms:      commsHandler,
		groupCount: len(cfg.Groups),
	}
}

// SetReceiver supplies the transport. Kept separate from construction so the
// receive endpoint can be configured after the handler exists, matching how the
// other adapters are assembled.
func (h *Handler) SetReceiver(rx receiver) { h.rx = rx }

// SetCommsHandler attaches the shared dispatcher.
func (h *Handler) SetCommsHandler(c *comms.Handler) { h.comms = c }

// SetVoteSink registers a callback for poll votes, which carry approvals. Until
// the approval channel is registered this stays nil and votes are only logged.
func (h *Handler) SetVoteSink(fn func(context.Context, signal.Vote)) { h.onVote = fn }

// SetSelfUUID records pilot's own Signal identity so its own traffic can be
// ignored. Without it, a message pilot sent could be read back as a command.
func (h *Handler) SetSelfUUID(uuid string) { h.selfUUID = uuid }

// StartListening blocks until ctx is cancelled or the transport fails fatally.
func (h *Handler) StartListening(ctx context.Context) error {
	if h.rx == nil {
		return errors.New("signalcli: no receiver configured")
	}
	h.log.Info("signal receive loop starting", "groups", h.groupCount)
	return h.rx.Run(ctx, func(f signal.Frame) { h.processFrame(ctx, f) })
}

// processFrame routes one frame. Most frames are neither commands nor votes —
// typing indicators, receipts and poll bookkeeping share the stream — so the
// allowlist and kind checks in the module decide, and anything else is dropped
// silently rather than logged at volume.
func (h *Handler) processFrame(ctx context.Context, f signal.Frame) {
	env := f.Envelope

	// Pilot's own messages must never round-trip into its command path.
	if h.selfUUID != "" && env.SourceUUID == h.selfUUID {
		return
	}

	if vote, ok := signal.VoteFrom(h.allow, env); ok {
		h.handleVote(ctx, vote)
		return
	}

	cmd, ok := signal.CommandFrom(h.allow, env)
	if !ok {
		return
	}
	if h.comms == nil {
		h.log.Warn("dropping signal command: no comms handler attached",
			"group", cmd.GroupID, "sender", cmd.SenderID)
		return
	}

	h.log.Debug("signal command received",
		"group", cmd.GroupID, "sender", cmd.SenderID,
		"text", comms.TruncateText(cmd.Text, 50))

	h.comms.HandleMessage(ctx, &comms.IncomingMessage{
		ContextID:  cmd.GroupID,
		SenderID:   cmd.SenderID,
		SenderName: env.SourceName,
		Text:       cmd.Text,
		Platform:   platform,
		Timestamp:  time.UnixMilli(cmd.Timestamp),
	})
}

// handleVote forwards a poll vote, which is how an approval arrives.
func (h *Handler) handleVote(ctx context.Context, vote signal.Vote) {
	h.log.Info("signal poll vote",
		"poll", vote.PollTimestamp, "voter", vote.VoterID,
		"options", vote.OptionIndexes, "revision", vote.Revision)
	if h.onVote == nil {
		return
	}
	h.onVote(ctx, vote)
}
