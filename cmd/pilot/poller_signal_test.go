package main

import (
	"testing"

	"github.com/qf-studio/pilot/internal/adapters/signalcli"
	"github.com/qf-studio/pilot/internal/config"
)

// A registration that is never listed is dead code that compiles, which is the
// failure mode this whole wiring step exists to avoid.
func TestSignalIsRegistered(t *testing.T) {
	var found bool
	for _, reg := range adapterPollerRegistrations() {
		if reg.Name == "signal" {
			found = true
		}
	}
	if !found {
		t.Fatal("signal adapter is not in adapterPollerRegistrations()")
	}
}

func TestSignalEnabledPredicate(t *testing.T) {
	reg := signalPollerRegistration()

	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{"nil adapter", &config.Config{}, false},
		{
			"present but disabled",
			&config.Config{Adapters: &config.AdaptersConfig{Signal: &signalcli.Config{Enabled: false}}},
			false,
		},
		{
			"enabled",
			&config.Config{Adapters: &config.AdaptersConfig{Signal: &signalcli.Config{Enabled: true}}},
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := reg.Enabled(tc.cfg); got != tc.want {
				t.Errorf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Defaults must leave the adapter off: enabling it without a group allowlist
// would mean listening with no authorization boundary.
func TestSignalDefaultsDisabled(t *testing.T) {
	d := signalcli.DefaultConfig()
	if d.Enabled {
		t.Error("signal adapter enabled by default")
	}
	if len(d.Groups) != 0 {
		t.Error("default config ships a group allowlist")
	}
}
