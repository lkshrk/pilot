package signal

import (
	"context"

	"github.com/qf-studio/pilot/internal/comms"
	sdkCore "github.com/qf-studio/studio-sdk/sdk/core"
)

// commsHandlerIface is the subset of *comms.Handler that Handler calls.
// Using an interface allows test doubles without a full comms stack.
type commsHandlerIface interface {
	HandleMessage(ctx context.Context, msg *comms.IncomingMessage)
	CleanupLoop(ctx context.Context)
}

// Handler is the core.MessageHandler for Signal: it converts normalized SDK
// events into comms.IncomingMessage and delegates to the shared comms.Handler.
type Handler struct {
	commsHandler commsHandlerIface
}

// NewHandler creates a Signal event handler. commsHandler may be nil and wired
// later with SetCommsHandler.
func NewHandler(commsHandler *comms.Handler) *Handler {
	var commsH commsHandlerIface
	if commsHandler != nil {
		commsH = commsHandler
	}
	return &Handler{commsHandler: commsH}
}

// HandleMessage implements core.MessageHandler.
//
// The conversion mirrors sdkshim.MessageEventToIncomingMessage; it is inlined
// here to avoid the sdkshim → config → signal → sdkshim import cycle.
func (h *Handler) HandleMessage(ctx context.Context, ev sdkCore.MessageEvent) error {
	msg := &comms.IncomingMessage{
		ContextID:  ev.ChannelID,
		SenderID:   ev.Sender.UserID,
		SenderName: ev.Sender.DisplayName,
		Text:       ev.Text,
		ThreadID:   ev.ThreadID,
		Platform:   "signal",
		RawEvent:   &ev,
	}
	if ev.Action == "callback" {
		msg.IsCallback = true
		msg.CallbackID = ev.CallbackID
		msg.ActionID = ev.Data
	}
	if h.commsHandler != nil {
		h.commsHandler.HandleMessage(ctx, msg)
	}
	return nil
}

// SetCommsHandler wires the shared comms handler after construction, breaking
// the bridge ↔ Messenger chicken-and-egg dependency.
func (h *Handler) SetCommsHandler(ch *comms.Handler) {
	h.commsHandler = ch
}
