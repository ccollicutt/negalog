package config

import (
	"os"
	"time"
)

// Default values for configuration.
const (
	DefaultTimeout          = 60 * time.Second
	DefaultMaxGap           = 5 * time.Minute
	DefaultWebhookTimeout   = 10 * time.Second
	DefaultTimestampPattern = `^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`
	DefaultTimestampLayout  = "2006-01-02 15:04:05"
)

// Environment variable names.
const (
	EnvLogSources      = "NEGALOG_LOG_SOURCES"
	EnvTimestampLayout = "NEGALOG_TIMESTAMP_LAYOUT"
)

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		LogSources: []string{},
		TimestampFormat: TimestampConfig{
			Pattern: DefaultTimestampPattern,
			Layout:  DefaultTimestampLayout,
		},
		Rules: []RuleConfig{},
	}
}

// applyEnvironmentOverrides applies environment variable overrides to the config.
func (c *Config) applyEnvironmentOverrides() {
	// Override timestamp layout from environment if set
	if layout := os.Getenv(EnvTimestampLayout); layout != "" {
		c.TimestampFormat.Layout = layout
	}
}
