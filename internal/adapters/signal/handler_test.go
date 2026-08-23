package signal

import (
	"context"
	"sync"
	"testing"

	"github.com/qf-studio/pilot/internal/comms"
	sdkCore "github.com/qf-studio/studio-sdk/sdk/core"
)

type mockCommsHandler struct {
	mu  sync.Mutex
	got []*comms.IncomingMessage
}

func (m *mockCommsHandler) HandleMessage(_ context.Context, msg *comms.IncomingMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.got = append(m.got, msg)
}

func (m *mockCommsHandler) CleanupLoop(_ context.Context) {}

func (m *mockCommsHandler) only(t *testing.T) *comms.IncomingMessage {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.got) != 1 {
		t.Fatalf("expected 1 delegated message, got %d", len(m.got))
	}
	return m.got[0]
}

func TestHandleMessage_PlainMessage(t *testing.T) {
	mock := &mockCommsHandler{}
	h := &Handler{commsHandler: mock}

	err := h.HandleMessage(context.Background(), sdkCore.MessageEvent{
		Action:    "message",
		ChannelID: "group.abc",
		Text:      "ship it",
		ThreadID:  "thread-1",
		Sender:    sdkCore.Identity{UserID: "uuid-1", DisplayName: "Ada"},
	})
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	msg := mock.only(t)
	if msg.Platform != "signal" {
		t.Errorf("Platform = %q, want signal", msg.Platform)
	}
	if msg.ContextID != "group.abc" {
		t.Errorf("ContextID = %q, want group.abc", msg.ContextID)
	}
	if msg.SenderID != "uuid-1" {
		t.Errorf("SenderID = %q, want uuid-1", msg.SenderID)
	}
	if msg.SenderName != "Ada" {
		t.Errorf("SenderName = %q, want Ada", msg.SenderName)
	}
	if msg.Text != "ship it" {
		t.Errorf("Text = %q, want ship it", msg.Text)
	}
	if msg.ThreadID != "thread-1" {
		t.Errorf("ThreadID = %q, want thread-1", msg.ThreadID)
	}
	if msg.IsCallback {
		t.Error("IsCallback = true, want false for a plain message")
	}
}

func TestHandleMessage_CallbackVote(t *testing.T) {
	mock := &mockCommsHandler{}
	h := &Handler{commsHandler: mock}

	err := h.HandleMessage(context.Background(), sdkCore.MessageEvent{
		Action:     "callback",
		ChannelID:  "group.abc",
		CallbackID: "poll-7",
		Data:       "execute",
		Sender:     sdkCore.Identity{UserID: "uuid-1"},
	})
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	msg := mock.only(t)
	if !msg.IsCallback {
		t.Error("IsCallback = false, want true")
	}
	if msg.CallbackID != "poll-7" {
		t.Errorf("CallbackID = %q, want poll-7", msg.CallbackID)
	}
	if msg.ActionID != "execute" {
		t.Errorf("ActionID = %q, want execute", msg.ActionID)
	}
}

func TestHandleMessage_NilCommsHandler(t *testing.T) {
	h := NewHandler(nil)
	if err := h.HandleMessage(context.Background(), sdkCore.MessageEvent{Text: "hi"}); err != nil {
		t.Fatalf("HandleMessage with nil comms handler: %v", err)
	}
}

func TestDefaultConfig_Disabled(t *testing.T) {
	if DefaultConfig().Enabled {
		t.Error("DefaultConfig().Enabled = true, want false")
	}
}
