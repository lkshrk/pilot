package main

import (
	"testing"

	"github.com/qf-studio/pilot/internal/adapters/signal"
	"github.com/qf-studio/pilot/internal/comms"
)

func TestSignalRateLimitToCommsNil(t *testing.T) {
	if got := signalRateLimitToComms(nil); got != nil {
		t.Errorf("signalRateLimitToComms(nil) = %+v, want nil", got)
	}
}

func TestSignalRateLimitToCommsConvertsUnits(t *testing.T) {
	got := signalRateLimitToComms(&signal.RateLimitConfig{
		MessagesPerSecond: 5,
		TasksPerMinute:    10,
	})
	if got == nil {
		t.Fatal("signalRateLimitToComms returned nil for a populated config")
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}
	if got.MessagesPerMinute != 300 {
		t.Errorf("MessagesPerMinute = %d, want 300", got.MessagesPerMinute)
	}
	if got.TasksPerHour != 600 {
		t.Errorf("TasksPerHour = %d, want 600", got.TasksPerHour)
	}
	if want := comms.DefaultRateLimitConfig().BurstSize; got.BurstSize != want {
		t.Errorf("BurstSize = %d, want %d", got.BurstSize, want)
	}
}
