package signalcli

// Config configures the Signal adapter.
type Config struct {
	Enabled bool `yaml:"enabled"`

	// BaseURL is the signal-cli-rest-api service root, e.g.
	// http://signal-rest-api.flimmerkiste.svc.cluster.local:80.
	BaseURL string `yaml:"base_url"`

	// Account is the registered Signal number this adapter sends and receives as.
	Account string `yaml:"account"`

	// Groups is the allowlist of Signal groups pilot will act on. Membership of
	// one of these groups is what grants a person approval authority, so this is
	// an authorization boundary rather than a noise filter: an empty list admits
	// nothing, which is the safe failure mode for a missing config.
	//
	// Either group encoding is accepted — the "group.<base64>" form returned by
	// GET /v1/groups, or the raw form carried in received envelopes.
	Groups []string `yaml:"groups"`

	// SelfUUID is this account's own Signal UUID, used to ignore pilot's own
	// traffic so a message it sent cannot be read back as a command. Optional;
	// without it self-filtering is skipped.
	SelfUUID string `yaml:"self_uuid"`

	// MaxMessageLength overrides the chunking threshold. Zero uses the module
	// default, which is conservative rather than API-derived — the API exposes
	// no limit to read.
	MaxMessageLength int `yaml:"max_message_length"`
}

// DefaultConfig returns the adapter disabled, which is the only safe default:
// enabling it without a group allowlist would leave pilot listening with no
// authorization boundary configured.
func DefaultConfig() *Config {
	return &Config{Enabled: false}
}
