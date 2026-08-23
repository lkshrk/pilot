// Package signal wires the studio-sdk Signal connector into pilot's comms stack.
package signal

// Config configures the Signal adapter.
type Config struct {
	Enabled bool `yaml:"enabled"`

	// BaseURL is the signal-cli-rest-api service root.
	BaseURL string `yaml:"base_url"`

	// Account is the registered Signal number this adapter sends and receives as.
	Account string `yaml:"account"`

	// Groups is an authorization boundary, not a noise filter: an empty list admits nothing.
	Groups []string `yaml:"groups"`

	// SelfUUID drops pilot's own traffic; without it self-filtering is skipped.
	SelfUUID string `yaml:"self_uuid"`

	// Approvers narrows who may decide an approval poll; empty keeps group membership as the boundary.
	Approvers []string `yaml:"approvers"`

	// StyledText sends messages with text_mode "styled".
	StyledText bool `yaml:"styled_text"`

	// MaxMessageLength overrides the chunking threshold; zero uses the SDK default.
	MaxMessageLength int `yaml:"max_message_length"`

	RateLimit *RateLimitConfig `yaml:"rate_limit"`
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	MessagesPerSecond int `yaml:"messages_per_second"`
	TasksPerMinute    int `yaml:"tasks_per_minute"`
}

// DefaultConfig returns the adapter disabled, the only safe default without a group allowlist.
func DefaultConfig() *Config {
	return &Config{Enabled: false}
}
