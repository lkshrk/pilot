package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ActiveAlert struct {
	Key         string
	RuleName    string
	AlertID     string
	AlertType   string
	Severity    string
	Title       string
	Message     string
	Source      string
	ProjectPath string
	Channels    []string
	CreatedAt   time.Time
}

func (s *Store) UpsertActiveAlert(a *ActiveAlert) error {
	if a.Key == "" {
		return fmt.Errorf("UpsertActiveAlert: Key must be set")
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	channels, err := json.Marshal(a.Channels)
	if err != nil {
		return fmt.Errorf("UpsertActiveAlert: marshal channels: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO active_alerts
			(key, rule_name, alert_id, alert_type, severity, title, message, source, project_path, channels, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Key, a.RuleName, a.AlertID, a.AlertType, a.Severity, a.Title,
		a.Message, a.Source, a.ProjectPath, string(channels), a.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("UpsertActiveAlert: %w", err)
	}
	return nil
}

func (s *Store) DeleteActiveAlert(key string) error {
	if _, err := s.db.Exec(`DELETE FROM active_alerts WHERE key = ?`, key); err != nil {
		return fmt.Errorf("DeleteActiveAlert: %w", err)
	}
	return nil
}

func (s *Store) LoadActiveAlerts() ([]*ActiveAlert, error) {
	rows, err := s.db.Query(`
		SELECT key, rule_name, alert_id, alert_type, severity, title, message, source, project_path, channels, created_at
		FROM active_alerts`)
	if err != nil {
		return nil, fmt.Errorf("LoadActiveAlerts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*ActiveAlert
	for rows.Next() {
		var a ActiveAlert
		var channels string
		if err := rows.Scan(&a.Key, &a.RuleName, &a.AlertID, &a.AlertType, &a.Severity,
			&a.Title, &a.Message, &a.Source, &a.ProjectPath, &channels, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("LoadActiveAlerts: scan: %w", err)
		}
		if strings.TrimSpace(channels) != "" {
			if err := json.Unmarshal([]byte(channels), &a.Channels); err != nil {
				return nil, fmt.Errorf("LoadActiveAlerts: unmarshal channels: %w", err)
			}
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}
