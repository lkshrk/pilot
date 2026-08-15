package executor

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type readOnlyAnswerBackend struct {
	gotProjectPath string
}

func (b *readOnlyAnswerBackend) Name() string      { return "readonly-answer" }
func (b *readOnlyAnswerBackend) IsAvailable() bool { return true }

func (b *readOnlyAnswerBackend) Execute(_ context.Context, opts ExecuteOptions) (*BackendResult, error) {
	b.gotProjectPath = opts.ProjectPath
	return &BackendResult{Success: true, Output: "the README says hello"}, nil
}

func TestRunner_QualityGates_SkippedForReadOnlyTasks(t *testing.T) {
	localRepo, remoteRepo := setupTestRepoWithRemote(t)
	defer func() { _ = os.RemoveAll(localRepo) }()
	defer func() { _ = os.RemoveAll(remoteRepo) }()

	var factoryCalls atomic.Int32
	backend := &readOnlyAnswerBackend{}
	runner := NewRunnerWithBackend(backend)
	runner.config = &BackendConfig{UseWorktree: false}
	runner.SetSkipPreflightChecks(true)
	runner.SetRecordingEnabled(false)
	runner.SetQualityCheckerFactory(func(taskID, projectPath string) QualityChecker {
		factoryCalls.Add(1)
		return &failingQualityChecker{}
	})

	task := &Task{
		ID:          "Q-1786817067",
		Title:       "Question: what does the README say",
		Description: "answer only, no changes",
		ProjectPath: localRepo,
		LocalMode:   true,
		ReadOnly:    true,
		CreatePR:    false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := runner.Execute(ctx, task)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("read-only task must succeed without gate interference, got %+v", result)
	}
	if got := factoryCalls.Load(); got != 0 {
		t.Errorf("quality checker factory invoked %d times for a ReadOnly task, want 0", got)
	}
	if backend.gotProjectPath != localRepo {
		t.Errorf("backend received ProjectPath %q, want %q", backend.gotProjectPath, localRepo)
	}
}
