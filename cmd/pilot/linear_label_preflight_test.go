package main

import (
	"context"
	"errors"
	"testing"

	"github.com/qf-studio/pilot/internal/adapters/linear"
)

type fakeScopeDiagnoser struct {
	scope linear.LabelScope
	err   error
	calls int
}

func (f *fakeScopeDiagnoser) DiagnoseLabelScope(_ context.Context, _, _ string) (linear.LabelScope, error) {
	f.calls++
	return f.scope, f.err
}

func TestDiagnoseTriggerLabel(t *testing.T) {
	tests := []struct {
		name  string
		scope linear.LabelScope
		err   error
	}{
		{name: "team-scoped is silent", scope: linear.LabelScopeTeam},
		{name: "workspace-scoped is reported", scope: linear.LabelScopeWorkspace},
		{name: "other team is reported", scope: linear.LabelScopeOtherTeam},
		{name: "missing is reported", scope: linear.LabelScopeMissing},
		{name: "query failure does not panic", err: errors.New("network down")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeScopeDiagnoser{scope: tt.scope, err: tt.err}
			diagnoseTriggerLabel(context.Background(), f, "acme", "ROU", "llm-pilot")
			if f.calls != 1 {
				t.Errorf("DiagnoseLabelScope called %d times, want 1", f.calls)
			}
		})
	}
}

func TestCheckLinearTriggerLabelsSkipsWhenDisabled(t *testing.T) {
	checkLinearTriggerLabels(context.Background(), nil)
}

type scriptedScopeDiagnoser struct {
	scopes map[string]linear.LabelScope
	err    error
	calls  []string
}

func (s *scriptedScopeDiagnoser) DiagnoseLabelScope(_ context.Context, _, labelName string) (linear.LabelScope, error) {
	s.calls = append(s.calls, labelName)
	if s.err != nil {
		return linear.LabelScopeMissing, s.err
	}
	if scope, ok := s.scopes[labelName]; ok {
		return scope, nil
	}
	return linear.LabelScopeMissing, nil
}

func TestDiagnoseStatusLabels(t *testing.T) {
	all := func(scope linear.LabelScope) map[string]linear.LabelScope {
		m := map[string]linear.LabelScope{}
		for _, l := range pilotStatusLabels {
			m[l] = scope
		}
		return m
	}

	tests := []struct {
		name      string
		scopes    map[string]linear.LabelScope
		err       error
		wantCalls int
	}{
		{name: "all team-scoped is silent", scopes: all(linear.LabelScopeTeam), wantCalls: 3},
		{name: "all workspace-scoped is reported", scopes: all(linear.LabelScopeWorkspace), wantCalls: 3},
		{name: "all missing is reported", scopes: map[string]linear.LabelScope{}, wantCalls: 3},
		{
			name: "one bad label is reported",
			scopes: map[string]linear.LabelScope{
				"pilot-in-progress": linear.LabelScopeWorkspace,
				"pilot-done":        linear.LabelScopeTeam,
				"pilot-failed":      linear.LabelScopeTeam,
			},
			wantCalls: 3,
		},
		{name: "query failure stops after the first label", err: errors.New("network down"), wantCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &scriptedScopeDiagnoser{scopes: tt.scopes, err: tt.err}
			diagnoseStatusLabels(context.Background(), d, "acme", "ROU")
			if len(d.calls) != tt.wantCalls {
				t.Errorf("checked %d labels (%v), want %d", len(d.calls), d.calls, tt.wantCalls)
			}
		})
	}
}

func TestPilotStatusLabelsMatchTheSDKSet(t *testing.T) {
	want := map[string]bool{"pilot-in-progress": true, "pilot-done": true, "pilot-failed": true}
	if len(pilotStatusLabels) != len(want) {
		t.Fatalf("pilotStatusLabels = %v, want the three labels the poller creates", pilotStatusLabels)
	}
	for _, l := range pilotStatusLabels {
		if !want[l] {
			t.Errorf("unexpected status label %q", l)
		}
	}
}
