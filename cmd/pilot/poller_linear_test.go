package main

import (
	"testing"
	"time"

	"github.com/qf-studio/pilot/internal/adapters/linear"
)

func TestNewSDKLinearWorkspace(t *testing.T) {
	tests := []struct {
		name           string
		ws             *linear.WorkspaceConfig
		triggerLabel   string
		interval       time.Duration
		wantProjectIDs []string
		wantProjects   []string
	}{
		{
			name: "project filter is handed to the SDK",
			ws: &linear.WorkspaceConfig{
				Name:       "acme",
				APIKey:     "test-linear-key",
				TeamID:     "ENG",
				ProjectIDs: []string{"proj-a", "proj-b"},
				Projects:   []string{"pilot"},
			},
			triggerLabel:   "llm-pilot",
			interval:       30 * time.Second,
			wantProjectIDs: []string{"proj-a", "proj-b"},
			wantProjects:   []string{"pilot"},
		},
		{
			name: "unset filter stays nil so the unfiltered path is unchanged",
			ws: &linear.WorkspaceConfig{
				Name:   "acme",
				APIKey: "test-linear-key",
				TeamID: "ENG",
			},
			triggerLabel:   "pilot",
			interval:       time.Minute,
			wantProjectIDs: nil,
			wantProjects:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newSDKLinearWorkspace(tt.ws, tt.triggerLabel, tt.interval)

			if got.ProjectIDs == nil && tt.wantProjectIDs != nil {
				t.Fatalf("ProjectIDs = nil, want %v", tt.wantProjectIDs)
			}
			if tt.wantProjectIDs == nil && got.ProjectIDs != nil {
				t.Errorf("ProjectIDs = %v, want nil", got.ProjectIDs)
			}
			if len(got.ProjectIDs) != len(tt.wantProjectIDs) {
				t.Fatalf("ProjectIDs = %v, want %v", got.ProjectIDs, tt.wantProjectIDs)
			}
			for i, want := range tt.wantProjectIDs {
				if got.ProjectIDs[i] != want {
					t.Errorf("ProjectIDs[%d] = %q, want %q", i, got.ProjectIDs[i], want)
				}
			}

			if tt.wantProjects == nil && got.Projects != nil {
				t.Errorf("Projects = %v, want nil", got.Projects)
			}
			if len(got.Projects) != len(tt.wantProjects) {
				t.Fatalf("Projects = %v, want %v", got.Projects, tt.wantProjects)
			}

			if got.Name != tt.ws.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.ws.Name)
			}
			if got.TeamID != tt.ws.TeamID {
				t.Errorf("TeamID = %q, want %q", got.TeamID, tt.ws.TeamID)
			}
			if got.TriggerLabel != tt.triggerLabel {
				t.Errorf("TriggerLabel = %q, want %q", got.TriggerLabel, tt.triggerLabel)
			}
			if got.Polling == nil || got.Polling.Interval != tt.interval {
				t.Errorf("Polling interval = %v, want %v", got.Polling, tt.interval)
			}
		})
	}
}
