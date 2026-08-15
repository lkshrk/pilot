package main

import (
	"context"
	"log/slog"

	"github.com/qf-studio/pilot/internal/adapters/linear"
	"github.com/qf-studio/pilot/internal/config"
	"github.com/qf-studio/pilot/internal/logging"
)

type labelScopeDiagnoser interface {
	DiagnoseLabelScope(ctx context.Context, teamRef, labelName string) (linear.LabelScope, error)
}

func checkLinearTriggerLabels(ctx context.Context, cfg *config.Config) {
	if cfg == nil || cfg.Adapters == nil || cfg.Adapters.Linear == nil || !cfg.Adapters.Linear.Enabled {
		return
	}
	for _, ws := range cfg.Adapters.Linear.GetWorkspaces() {
		if ws.APIKey == "" || ws.TeamID == "" {
			continue
		}
		label := ws.PilotLabel
		if label == "" {
			label = "pilot"
		}
		client := linear.NewClient(ws.APIKey)
		diagnoseTriggerLabel(ctx, client, ws.Name, ws.TeamID, label)
		diagnoseStatusLabels(ctx, client, ws.Name, ws.TeamID)
	}
}

func diagnoseTriggerLabel(ctx context.Context, client labelScopeDiagnoser, wsName, teamRef, label string) {
	log := logging.WithComponent("linear-preflight")

	scope, err := client.DiagnoseLabelScope(ctx, teamRef, label)
	if err != nil {
		log.Warn("could not verify the Linear trigger label",
			slog.String("workspace", wsName), slog.String("team", teamRef),
			slog.String("label", label), slog.Any("error", err))
		return
	}

	switch scope {
	case linear.LabelScopeTeam:
		return
	case linear.LabelScopeWorkspace:
		log.Error("Linear trigger label is workspace-scoped; the poller only matches team-scoped labels and will find no issues. Label scope is immutable — delete it, recreate it scoped to the team, and re-apply it to affected issues",
			slog.String("workspace", wsName), slog.String("team", teamRef), slog.String("label", label))
	case linear.LabelScopeOtherTeam:
		log.Error("Linear trigger label exists only on a different team; create it on the configured team",
			slog.String("workspace", wsName), slog.String("team", teamRef), slog.String("label", label))
	case linear.LabelScopeMissing:
		log.Error("Linear trigger label does not exist; pilot never auto-creates it, create it scoped to the team",
			slog.String("workspace", wsName), slog.String("team", teamRef), slog.String("label", label))
	}
}

var pilotStatusLabels = []string{"pilot-in-progress", "pilot-done", "pilot-failed"}

func diagnoseStatusLabels(ctx context.Context, client labelScopeDiagnoser, wsName, teamRef string) {
	log := logging.WithComponent("linear-preflight")

	var unusable []string
	for _, label := range pilotStatusLabels {
		scope, err := client.DiagnoseLabelScope(ctx, teamRef, label)
		if err != nil {
			log.Warn("could not verify a Linear status label",
				slog.String("workspace", wsName), slog.String("team", teamRef),
				slog.String("label", label), slog.Any("error", err))
			return
		}
		if scope != linear.LabelScopeTeam {
			unusable = append(unusable, label)
		}
	}
	if len(unusable) == 0 {
		return
	}

	log.Error("Linear status labels are missing or not team-scoped and cannot be created automatically: label creation requires the team UUID while issue and label lookup require the team key, so the poller logs a warning and then runs without them — issues will execute but will never be marked in-progress, done, or failed. Create these labels scoped to the team",
		slog.String("workspace", wsName), slog.String("team", teamRef),
		slog.Any("labels", unusable))
}
