package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/qf-studio/pilot/internal/adapters/sdkshim"
	"github.com/qf-studio/pilot/internal/adapters/signal"
	"github.com/qf-studio/pilot/internal/comms"
	"github.com/qf-studio/pilot/internal/config"
	"github.com/qf-studio/pilot/internal/logging"
	sdkCore "github.com/qf-studio/studio-sdk/sdk/core"
	sdkSignal "github.com/qf-studio/studio-sdk/sdk/integrations/signal"
)

func signalPollerRegistration() PollerRegistration {
	return PollerRegistration{
		Name: "signal",
		Enabled: func(cfg *config.Config) bool {
			return cfg.Adapters.Signal != nil && cfg.Adapters.Signal.Enabled
		},
		CreateAndStart: func(ctx context.Context, deps *PollerDeps) {
			signalCfg := deps.Cfg.Adapters.Signal

			var signalBotCfg *comms.BotConfig
			if deps.Cfg.Bot != nil {
				signalBotCfg = &comms.BotConfig{
					Enabled:     deps.Cfg.Bot.Enabled,
					Model:       deps.Cfg.Bot.Model,
					AnswerModel: deps.Cfg.Bot.AnswerModel,
					APIKey:      deps.Cfg.Bot.APIKey,
					Persona:     deps.Cfg.Bot.Persona,
				}
			}

			// SetCommsHandler is called after the bridge messenger is created to
			// break the bridge ↔ Messenger circular dependency.
			signalChatHandler := signal.NewHandler(nil)

			signalBridge := sdkSignal.New(sdkSignal.Config{
				BaseURL:          signalCfg.BaseURL,
				Account:          signalCfg.Account,
				Groups:           signalCfg.Groups,
				SelfUUID:         signalCfg.SelfUUID,
				Approvers:        signalCfg.Approvers,
				StyledText:       signalCfg.StyledText,
				MaxMessageLength: signalCfg.MaxMessageLength,
			}, nil).NewChatBridge(sdkCore.ChatDeps{Handler: signalChatHandler})

			signalCommsHandler := comms.BuildHandler(comms.HandlerDeps{
				Messenger:       sdkshim.MessengerToBridge(signalBridge),
				Runner:          deps.Runner,
				Projects:        config.NewProjectSource(deps.Cfg),
				ProjectPath:     deps.ProjectPath,
				Bot:             signalBotCfg,
				Store:           deps.Store,
				TaskIDPrefix:    "SIGNAL",
				ExecutorBackend: deps.Cfg.Executor,
			})
			signalChatHandler.SetCommsHandler(signalCommsHandler)

			deps.SafeAdapterGo(ctx, "signal", func() {
				if err := signalBridge.Start(ctx); err != nil {
					logging.WithComponent("signal").Error("Signal listener error",
						slog.Any("error", err),
					)
				}
			})
			fmt.Println("● signal bot started")
			logging.WithComponent("start").Info("Signal bot started")
		},
	}
}
