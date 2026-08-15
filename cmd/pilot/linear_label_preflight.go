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
		log.Warn("Linear trigger label is workspace-scoped; team-filtered label lookup only finds it when the SDK falls back to workspace labels. If the poller fails at startup with a label-not-found error: label scope is immutable — delete it, recreate it scoped to the team, and re-apply it to affected issues",
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

	var missing, wrongScope []string
	for _, label := range pilotStatusLabels {
		scope, err := client.DiagnoseLabelScope(ctx, teamRef, label)
		if err != nil {
			log.Warn("could not verify a Linear status label",
				slog.String("workspace", wsName), slog.String("team", teamRef),
				slog.String("label", label), slog.Any("error", err))
			return
		}
		switch scope {
		case linear.LabelScopeMissing:
			missing = append(missing, label)
		case linear.LabelScopeWorkspace, linear.LabelScopeOtherTeam:
			wrongScope = append(wrongScope, label)
		}
	}

	if len(missing) > 0 {
		log.Info("Linear status labels do not exist yet; the poller attempts to create them at startup. If they are still missing after the first poll, creation failed (label creation needs the team UUID) — create them by hand, scoped to the team",
			slog.String("workspace", wsName), slog.String("team", teamRef),
			slog.Any("labels", missing))
	}
	if len(wrongScope) > 0 {
		log.Warn("Linear status labels exist but are not scoped to the configured team; the poller only marks issues with them when the SDK resolves labels outside the team scope. If issues stop being marked in-progress, done, or failed: label scope is immutable — delete and recreate these scoped to the team",
			slog.String("workspace", wsName), slog.String("team", teamRef),
			slog.Any("labels", wrongScope))
	}
}
