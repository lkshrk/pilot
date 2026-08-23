package signal

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigUnmarshalRateLimit(t *testing.T) {
	const src = `
enabled: true
base_url: http://signal-rest-api:8080
account: "+4915112345678"
rate_limit:
  messages_per_second: 3
  tasks_per_minute: 7
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.RateLimit == nil {
		t.Fatal("RateLimit is nil, want parsed block")
	}
	if got := cfg.RateLimit.MessagesPerSecond; got != 3 {
		t.Errorf("MessagesPerSecond = %d, want 3", got)
	}
	if got := cfg.RateLimit.TasksPerMinute; got != 7 {
		t.Errorf("TasksPerMinute = %d, want 7", got)
	}
}

func TestConfigUnmarshalWithoutRateLimit(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("enabled: true\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.RateLimit != nil {
		t.Errorf("RateLimit = %+v, want nil when the block is absent", cfg.RateLimit)
	}
}
