package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	signal "github.com/lkshrk/pilot-signal-adapter/signalcli"
	"github.com/qf-studio/pilot/internal/adapters/signalcli"
	"github.com/qf-studio/pilot/internal/comms"
	"github.com/qf-studio/pilot/internal/config"
	"github.com/qf-studio/pilot/internal/logging"
)

// signalPollerRegistration wires the Signal adapter.
//
// Unlike discord, this adapter does not go through the studio-sdk bridge: the
// Signal messenger implements comms.Messenger directly, so it is handed to
// comms.BuildHandler without a shim.
func signalPollerRegistration() PollerRegistration {
	return PollerRegistration{
		Name: "signal",
		Enabled: func(cfg *config.Config) bool {
			return cfg != nil && cfg.Adapters != nil &&
				cfg.Adapters.Signal != nil && cfg.Adapters.Signal.Enabled
		},
		CreateAndStart: func(ctx context.Context, deps *PollerDeps) {
			cfg := deps.Cfg.Adapters.Signal
			log := logging.WithComponent("signal")

			// An enabled adapter with no groups would listen with no
			// authorization boundary, so refuse rather than start permissively.
			if len(cfg.Groups) == 0 {
				log.Error("signal adapter enabled with no groups configured; refusing to start")
				return
			}

			opts := []signal.SenderOption{}
			if cfg.MaxMessageLength > 0 {
				opts = append(opts, signal.WithMaxMessageLength(cfg.MaxMessageLength))
			}
			sender, err := signal.NewSender(cfg.BaseURL, cfg.Account, opts...)
			if err != nil {
				log.Error("signal sender config invalid", slog.Any("error", err))
				return
			}

			receiver, err := signal.New(cfg.BaseURL, cfg.Account,
				signal.WithLogger(log),
				signal.WithOnReconnect(func(attempt int, connectedFor time.Duration, cause error) {
					// Messages arriving while disconnected are dropped, not
					// queued, so every reconnect is a loss window worth seeing.
					log.Warn("signal receive gap",
						slog.Int("attempt", attempt),
						slog.Duration("connected_for", connectedFor),
						slog.Any("cause", cause))
				}),
			)
			if err != nil {
				log.Error("signal receiver config invalid", slog.Any("error", err))
				return
			}

			messenger := signalcli.NewMessenger(sender)

			handler := signalcli.NewHandler(&signalcli.HandlerConfig{
				Groups: cfg.Groups,
				Logger: log,
			}, nil)
			handler.SetReceiver(receiver)
			handler.SetSelfUUID(cfg.SelfUUID)

			var botCfg *comms.BotConfig
			if deps.Cfg.Bot != nil {
				botCfg = &comms.BotConfig{
					Enabled:     deps.Cfg.Bot.Enabled,
					Model:       deps.Cfg.Bot.Model,
					AnswerModel: deps.Cfg.Bot.AnswerModel,
					APIKey:      deps.Cfg.Bot.APIKey,
					Persona:     deps.Cfg.Bot.Persona,
				}
			}

			handler.SetCommsHandler(comms.BuildHandler(comms.HandlerDeps{
				Messenger:       messenger,
				Runner:          deps.Runner,
				Projects:        config.NewProjectSource(deps.Cfg),
				ProjectPath:     deps.ProjectPath,
				Bot:             botCfg,
				Store:           deps.Store,
				TaskIDPrefix:    "SIGNAL",
				ExecutorBackend: deps.Cfg.Executor,
			}))

			deps.SafeAdapterGo(ctx, "signal", func() {
				if err := handler.StartListening(ctx); err != nil {
					log.Error("signal listener stopped", slog.Any("error", err))
				}
			})
			fmt.Println("● signal bot started")
			logging.WithComponent("start").Info("Signal bot started")
		},
	}
}
