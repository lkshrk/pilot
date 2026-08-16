// Package signalcli adapts the Signal transport to pilot's comms interfaces.
//
// It stays deliberately thin: transport, parsing and the Signal API live in
// github.com/lkshrk/pilot-signal-adapter, which cannot import pilot. Everything
// here is fork-local and therefore rebase surface, so logic that can live in the
// module belongs there.
package signalcli

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	signal "github.com/lkshrk/pilot-signal-adapter/signalcli"
)

// approveIndex and rejectIndex are the poll option positions. Signal reports a
// vote as an index, so the order here defines what an approval means.
const (
	approveIndex = 0
	rejectIndex  = 1
)

var pollOptions = []string{"Approve", "Reject"}

// sender is the subset of the module's Sender this adapter uses, named so tests
// can substitute it.
type sender interface {
	SendText(ctx context.Context, recipient, text string) (int64, error)
	SendChunked(ctx context.Context, recipient, text string) ([]int64, error)
	CreatePoll(ctx context.Context, recipient, question string, options []string, allowMultiple bool) (int64, error)
	ClosePoll(ctx context.Context, recipient string, pollTimestamp int64) error
	MaxMessageLength() int
}

// Messenger implements comms.Messenger over Signal.
//
// Signal has no threads and no message editing, so threadID is ignored and
// progress updates post a new message rather than mutating one.
type Messenger struct {
	api sender

	// pending maps a confirmation's messageRef to the group its poll was posted
	// in. AcknowledgeCallback receives only the ref, but closing a poll needs
	// the recipient too, and closing on the first vote is what makes an approval
	// final.
	mu      sync.Mutex
	pending map[string]string
}

// NewMessenger builds a Messenger over the given sender.
func NewMessenger(api sender) *Messenger {
	return &Messenger{api: api, pending: make(map[string]string)}
}

// recipient normalises a pilot context ID into the form the send endpoints
// expect. Group identifiers appear in two encodings and pilot's config may carry
// either, so converting here keeps that off every call site.
func recipient(contextID string) string {
	return signal.GroupRecipient(contextID)
}

// SendText sends a plain message. threadID is ignored: Signal has no threads.
func (m *Messenger) SendText(ctx context.Context, contextID, _ /*threadID*/, text string) error {
	if _, err := m.api.SendText(ctx, recipient(contextID), text); err != nil {
		return fmt.Errorf("signalcli: send text: %w", err)
	}
	return nil
}

// SendConfirmation posts the approval prompt as a Signal poll and returns the
// poll timestamp as the messageRef.
//
// Signal has no inline buttons, and a poll is the closest equivalent: the vote
// binds to this specific poll, the voter is identifiable, and closing the poll
// makes the answer final. Reply-text and emoji reactions were both considered
// and are weaker on binding and auditability.
func (m *Messenger) SendConfirmation(ctx context.Context, contextID, _ /*threadID*/, taskID, desc, project string) (string, error) {
	to := recipient(contextID)
	question := confirmationQuestion(taskID, desc, project)

	ts, err := m.api.CreatePoll(ctx, to, question, pollOptions, false)
	if err != nil {
		return "", fmt.Errorf("signalcli: send confirmation: %w", err)
	}

	ref := strconv.FormatInt(ts, 10)
	m.mu.Lock()
	m.pending[ref] = to
	m.mu.Unlock()
	return ref, nil
}

// SendProgress posts an update. Signal cannot edit a sent message, so this
// creates a new one and returns its ref; the previous ref stays valid for the
// poll it identified.
func (m *Messenger) SendProgress(ctx context.Context, contextID, _ /*messageRef*/, taskID, phase string, progress int, detail string) (string, error) {
	ts, err := m.api.SendText(ctx, recipient(contextID), progressText(taskID, phase, progress, detail))
	if err != nil {
		return "", fmt.Errorf("signalcli: send progress: %w", err)
	}
	return strconv.FormatInt(ts, 10), nil
}

// SendResult reports the outcome.
func (m *Messenger) SendResult(ctx context.Context, contextID, _ /*threadID*/, taskID string, success bool, output, prURL string) error {
	if _, err := m.api.SendText(ctx, recipient(contextID), resultText(taskID, success, output, prURL)); err != nil {
		return fmt.Errorf("signalcli: send result: %w", err)
	}
	return nil
}

// SendChunked splits long content at the platform limit.
func (m *Messenger) SendChunked(ctx context.Context, contextID, _ /*threadID*/, content, prefix string) error {
	body := content
	if prefix != "" {
		body = prefix + "\n\n" + content
	}
	if _, err := m.api.SendChunked(ctx, recipient(contextID), body); err != nil {
		return fmt.Errorf("signalcli: send chunked: %w", err)
	}
	return nil
}

// AcknowledgeCallback closes the poll behind a confirmation, which is what makes
// the answer final — Signal allows an open poll's vote to be changed, so leaving
// it open would let an approval be revised after the merge it authorised.
//
// An unknown ref is not an error: pilot acknowledges callbacks that never came
// from a poll, and failing those would turn a no-op into a spurious failure.
func (m *Messenger) AcknowledgeCallback(ctx context.Context, callbackID string) error {
	m.mu.Lock()
	to, ok := m.pending[callbackID]
	if ok {
		delete(m.pending, callbackID)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}

	ts, err := strconv.ParseInt(callbackID, 10, 64)
	if err != nil {
		return nil
	}
	if err := m.api.ClosePoll(ctx, to, ts); err != nil {
		// Restore it: the approval is not final, and a retry must be able to
		// find the group again.
		m.mu.Lock()
		m.pending[callbackID] = to
		m.mu.Unlock()
		return fmt.Errorf("signalcli: close approval poll: %w", err)
	}
	return nil
}

// MaxMessageLength reports the chunking threshold.
func (m *Messenger) MaxMessageLength() int { return m.api.MaxMessageLength() }
