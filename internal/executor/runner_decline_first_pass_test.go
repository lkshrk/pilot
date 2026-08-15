package executor

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type decliningBackend struct {
	calls atomic.Int32
}

func (b *decliningBackend) Name() string      { return "declining" }
func (b *decliningBackend) IsAvailable() bool { return true }

func (b *decliningBackend) Execute(_ context.Context, _ ExecuteOptions) (*BackendResult, error) {
	b.calls.Add(1)
	return &BackendResult{
		Success:           true,
		Output:            "analysis complete\nDECLINED: the requested feature already exists in internal/auth/jwt.go",
		LastAssistantText: "analysis complete\nDECLINED: the requested feature already exists in internal/auth/jwt.go",
	}, nil
}

func TestRunner_FirstPassDecline_SkipsNoCommitRetry(t *testing.T) {
	localRepo, remoteRepo := setupTestRepoWithRemote(t)
	defer func() { _ = os.RemoveAll(localRepo) }()
	defer func() { _ = os.RemoveAll(remoteRepo) }()

	backend := &decliningBackend{}
	runner := NewRunnerWithBackend(backend)
	runner.config = &BackendConfig{UseWorktree: false}
	runner.SetSkipPreflightChecks(true)
	runner.SetRecordingEnabled(false)

	task := &Task{
		ID:          "GH-4875-1",
		Title:       "add auth module",
		Description: "already exists; executor declines on the first pass",
		ProjectPath: localRepo,
		Branch:      "pilot/GH-4875-1",
		CreatePR:    true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := runner.Execute(ctx, task)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil || !result.Declined {
		t.Fatalf("expected declined result, got %+v", result)
	}
	if result.Success {
		t.Error("declined result must not report Success")
	}
	if result.Outcome != "declined" {
		t.Errorf("Outcome = %q, want \"declined\"", result.Outcome)
	}
	if result.DeclinedReason == "" {
		t.Error("DeclinedReason must carry the DECLINED: text")
	}
	if got := backend.calls.Load(); got != 1 {
		t.Errorf("backend invoked %d times, want 1 — a first-pass decline must not trigger the no-commit retry", got)
	}
}
