package memory

import (
	"testing"
	"time"
)

func TestActiveAlertRoundTrip(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	created := time.Now().UTC().Truncate(time.Second)
	in := &ActiveAlert{
		Key:         "config-error|adapter:github",
		RuleName:    "config-error",
		AlertID:     "alert-1",
		AlertType:   "service_unhealthy",
		Severity:    "warning",
		Title:       "Adapter health",
		Message:     "github adapter verification failed",
		Source:      "adapter:github",
		ProjectPath: "/repo",
		Channels:    []string{"tg-critical", "tg-routine"},
		CreatedAt:   created,
	}

	if err := s.UpsertActiveAlert(in); err != nil {
		t.Fatalf("UpsertActiveAlert: %v", err)
	}

	got, err := s.LoadActiveAlerts()
	if err != nil {
		t.Fatalf("LoadActiveAlerts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("loaded %d rows, want 1", len(got))
	}

	a := got[0]
	if a.Key != in.Key || a.RuleName != in.RuleName || a.AlertID != in.AlertID {
		t.Errorf("identity mismatch: %+v", a)
	}
	if a.AlertType != in.AlertType || a.Severity != in.Severity {
		t.Errorf("type/severity mismatch: %q %q", a.AlertType, a.Severity)
	}
	if a.Title != in.Title || a.Message != in.Message || a.Source != in.Source || a.ProjectPath != in.ProjectPath {
		t.Errorf("payload mismatch: %+v", a)
	}
	if len(a.Channels) != 2 || a.Channels[0] != "tg-critical" || a.Channels[1] != "tg-routine" {
		t.Errorf("channels = %v, want [tg-critical tg-routine]", a.Channels)
	}
	if !a.CreatedAt.UTC().Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", a.CreatedAt.UTC(), created)
	}

	if err := s.DeleteActiveAlert(in.Key); err != nil {
		t.Fatalf("DeleteActiveAlert: %v", err)
	}
	got, err = s.LoadActiveAlerts()
	if err != nil {
		t.Fatalf("LoadActiveAlerts after delete: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("loaded %d rows after delete, want 0", len(got))
	}
}

func TestUpsertActiveAlertReplacesSameKey(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	base := &ActiveAlert{
		Key: "r|adapter:github", RuleName: "r", AlertID: "first",
		AlertType: "service_unhealthy", Severity: "warning", Source: "adapter:github",
	}
	if err := s.UpsertActiveAlert(base); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	base.AlertID = "second"
	if err := s.UpsertActiveAlert(base); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := s.LoadActiveAlerts()
	if err != nil {
		t.Fatalf("LoadActiveAlerts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("loaded %d rows, want 1", len(got))
	}
	if got[0].AlertID != "second" {
		t.Errorf("AlertID = %q, want second", got[0].AlertID)
	}
}

func TestActiveAlertIndependentSources(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	for _, src := range []string{"adapter:github", "adapter:linear"} {
		if err := s.UpsertActiveAlert(&ActiveAlert{
			Key: "r|" + src, RuleName: "r", AlertID: "a-" + src,
			AlertType: "service_unhealthy", Severity: "warning", Source: src,
		}); err != nil {
			t.Fatalf("upsert %s: %v", src, err)
		}
	}

	if err := s.DeleteActiveAlert("r|adapter:github"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, err := s.LoadActiveAlerts()
	if err != nil {
		t.Fatalf("LoadActiveAlerts: %v", err)
	}
	if len(got) != 1 || got[0].Source != "adapter:linear" {
		t.Fatalf("remaining rows = %+v, want only adapter:linear", got)
	}
}

func TestUpsertActiveAlertRequiresKey(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	if err := s.UpsertActiveAlert(&ActiveAlert{RuleName: "r"}); err == nil {
		t.Fatal("expected error for empty Key")
	}
}

func TestUpsertActiveAlertDefaultsCreatedAt(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	if err := s.UpsertActiveAlert(&ActiveAlert{
		Key: "r|s", RuleName: "r", AlertID: "a",
		AlertType: "service_unhealthy", Severity: "warning", Source: "s",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.LoadActiveAlerts()
	if err != nil {
		t.Fatalf("LoadActiveAlerts: %v", err)
	}
	if got[0].CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want defaulted to now")
	}
}
