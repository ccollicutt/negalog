// Package config provides configuration loading and validation for NegaLog.
package config

import (
	"regexp"
	"time"
)

// Config is the root configuration structure loaded from YAML.
type Config struct {
	LogSources      []string        `yaml:"log_sources"`
	TimestampFormat TimestampConfig `yaml:"timestamp_format"`
	Rules           []RuleConfig    `yaml:"rules"`
	Webhooks        []WebhookConfig `yaml:"webhooks,omitempty"`
}

// TimestampConfig defines how to extract timestamps from log lines.
type TimestampConfig struct {
	// Pattern is a regex that captures the timestamp portion of a log line.
	// Must contain at least one capture group.
	Pattern string `yaml:"pattern"`

	// Layout is the Go time layout string for parsing the captured timestamp.
	// See https://pkg.go.dev/time#pkg-constants for format.
	Layout string `yaml:"layout"`

	// compiledPattern is the pre-compiled regex (populated during validation).
	compiledPattern *regexp.Regexp
}

// CompiledPattern returns the pre-compiled regex pattern.
func (t *TimestampConfig) CompiledPattern() *regexp.Regexp {
	return t.compiledPattern
}

// RuleType represents the type of detection rule.
type RuleType string

const (
	RuleTypeSequence    RuleType = "sequence"
	RuleTypePeriodic    RuleType = "periodic"
	RuleTypeConditional RuleType = "conditional"
)

// RuleConfig defines a single detection rule.
type RuleConfig struct {
	// Common fields
	Name        string `yaml:"name"`
	Type        string `yaml:"type"` // sequence, periodic, conditional
	Description string `yaml:"description,omitempty"`

	// Sequence rule fields
	StartPattern     string        `yaml:"start_pattern,omitempty"`
	EndPattern       string        `yaml:"end_pattern,omitempty"`
	CorrelationField int           `yaml:"correlation_field,omitempty"` // capture group index (1-based)
	Timeout          time.Duration `yaml:"timeout,omitempty"`

	// Periodic rule fields
	Pattern        string        `yaml:"pattern,omitempty"`
	MaxGap         time.Duration `yaml:"max_gap,omitempty"`
	MinOccurrences int           `yaml:"min_occurrences,omitempty"`

	// Conditional rule fields
	TriggerPattern  string `yaml:"trigger_pattern,omitempty"`
	ExpectedPattern string `yaml:"expected_pattern,omitempty"`
	// Timeout is shared with sequence rules
	// CorrelationField is shared with sequence rules

	// Compiled patterns (populated during validation)
	compiledStartPattern    *regexp.Regexp
	compiledEndPattern      *regexp.Regexp
	compiledPattern         *regexp.Regexp
	compiledTriggerPattern  *regexp.Regexp
	compiledExpectedPattern *regexp.Regexp
}

// CompiledStartPattern returns the compiled start pattern for sequence rules.
func (r *RuleConfig) CompiledStartPattern() *regexp.Regexp {
	return r.compiledStartPattern
}

// CompiledEndPattern returns the compiled end pattern for sequence rules.
func (r *RuleConfig) CompiledEndPattern() *regexp.Regexp {
	return r.compiledEndPattern
}

// CompiledPattern returns the compiled pattern for periodic rules.
func (r *RuleConfig) CompiledPattern() *regexp.Regexp {
	return r.compiledPattern
}

// CompiledTriggerPattern returns the compiled trigger pattern for conditional rules.
func (r *RuleConfig) CompiledTriggerPattern() *regexp.Regexp {
	return r.compiledTriggerPattern
}

// CompiledExpectedPattern returns the compiled expected pattern for conditional rules.
func (r *RuleConfig) CompiledExpectedPattern() *regexp.Regexp {
	return r.compiledExpectedPattern
}

// RuleTypeEnum returns the rule type as a RuleType enum.
func (r *RuleConfig) RuleTypeEnum() RuleType {
	return RuleType(r.Type)
}

// WebhookTrigger determines when a webhook fires.
type WebhookTrigger string

const (
	// WebhookTriggerOnIssues fires only when issues are detected (default).
	WebhookTriggerOnIssues WebhookTrigger = "on_issues"
	// WebhookTriggerAlways fires after every analysis.
	WebhookTriggerAlways WebhookTrigger = "always"
	// WebhookTriggerNever disables the webhook.
	WebhookTriggerNever WebhookTrigger = "never"
)

// WebhookConfig defines a webhook endpoint for sending analysis results.
type WebhookConfig struct {
	// Name is an optional identifier for the webhook.
	Name string `yaml:"name,omitempty"`

	// URL is the webhook endpoint (required).
	URL string `yaml:"url"`

	// Token is an optional bearer token for authentication.
	Token string `yaml:"token,omitempty"`

	// Trigger determines when the webhook fires.
	// Defaults to "on_issues" if not specified.
	Trigger WebhookTrigger `yaml:"trigger,omitempty"`

	// Timeout is the HTTP request timeout.
	// Defaults to 10s if not specified.
	Timeout time.Duration `yaml:"timeout,omitempty"`
}
