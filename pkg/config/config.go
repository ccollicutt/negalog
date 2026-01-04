package config

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and validates a configuration file.
func Load(_ context.Context, path string) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- user-provided config path is expected
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	cfg.applyEnvironmentOverrides()

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// Validate checks a configuration for errors and compiles regex patterns.
func Validate(cfg *Config) error {
	if len(cfg.LogSources) == 0 {
		return errors.New("log_sources: at least one log source is required")
	}

	if err := validateTimestampFormat(&cfg.TimestampFormat); err != nil {
		return fmt.Errorf("timestamp_format: %w", err)
	}

	if len(cfg.Rules) == 0 {
		return errors.New("rules: at least one rule is required")
	}

	for i := range cfg.Rules {
		if err := validateRule(&cfg.Rules[i]); err != nil {
			return fmt.Errorf("rules[%d] (%s): %w", i, cfg.Rules[i].Name, err)
		}
	}

	// Webhooks are optional, but validate if present
	for i := range cfg.Webhooks {
		if err := validateWebhook(&cfg.Webhooks[i]); err != nil {
			name := cfg.Webhooks[i].Name
			if name == "" {
				name = cfg.Webhooks[i].URL
			}
			return fmt.Errorf("webhooks[%d] (%s): %w", i, name, err)
		}
	}

	return nil
}

func validateTimestampFormat(tf *TimestampConfig) error {
	if tf.Pattern == "" {
		return errors.New("pattern is required")
	}

	re, err := regexp.Compile(tf.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	if re.NumSubexp() < 1 {
		return errors.New("pattern must have at least one capture group for the timestamp")
	}

	tf.compiledPattern = re

	if tf.Layout == "" {
		return errors.New("layout is required")
	}

	return nil
}

func validateRule(rule *RuleConfig) error {
	if rule.Name == "" {
		return errors.New("name is required")
	}

	switch RuleType(rule.Type) {
	case RuleTypeSequence:
		return validateSequenceRule(rule)
	case RuleTypePeriodic:
		return validatePeriodicRule(rule)
	case RuleTypeConditional:
		return validateConditionalRule(rule)
	default:
		return fmt.Errorf("invalid type %q (must be sequence, periodic, or conditional)", rule.Type)
	}
}

func validateSequenceRule(rule *RuleConfig) error {
	if rule.StartPattern == "" {
		return errors.New("start_pattern is required for sequence rules")
	}

	re, err := regexp.Compile(rule.StartPattern)
	if err != nil {
		return fmt.Errorf("invalid start_pattern: %w", err)
	}
	rule.compiledStartPattern = re

	if rule.EndPattern == "" {
		return errors.New("end_pattern is required for sequence rules")
	}

	re, err = regexp.Compile(rule.EndPattern)
	if err != nil {
		return fmt.Errorf("invalid end_pattern: %w", err)
	}
	rule.compiledEndPattern = re

	if rule.CorrelationField < 1 {
		return errors.New("correlation_field must be >= 1 (capture group index)")
	}

	if rule.compiledStartPattern.NumSubexp() < rule.CorrelationField {
		return fmt.Errorf("start_pattern has only %d capture groups, but correlation_field is %d",
			rule.compiledStartPattern.NumSubexp(), rule.CorrelationField)
	}

	if rule.compiledEndPattern.NumSubexp() < rule.CorrelationField {
		return fmt.Errorf("end_pattern has only %d capture groups, but correlation_field is %d",
			rule.compiledEndPattern.NumSubexp(), rule.CorrelationField)
	}

	if rule.Timeout <= 0 {
		rule.Timeout = DefaultTimeout
	}

	return nil
}

func validatePeriodicRule(rule *RuleConfig) error {
	if rule.Pattern == "" {
		return errors.New("pattern is required for periodic rules")
	}

	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}
	rule.compiledPattern = re

	if rule.MaxGap <= 0 {
		rule.MaxGap = DefaultMaxGap
	}

	return nil
}

func validateConditionalRule(rule *RuleConfig) error {
	if rule.TriggerPattern == "" {
		return errors.New("trigger_pattern is required for conditional rules")
	}

	re, err := regexp.Compile(rule.TriggerPattern)
	if err != nil {
		return fmt.Errorf("invalid trigger_pattern: %w", err)
	}
	rule.compiledTriggerPattern = re

	if rule.ExpectedPattern == "" {
		return errors.New("expected_pattern is required for conditional rules")
	}

	re, err = regexp.Compile(rule.ExpectedPattern)
	if err != nil {
		return fmt.Errorf("invalid expected_pattern: %w", err)
	}
	rule.compiledExpectedPattern = re

	if rule.Timeout <= 0 {
		rule.Timeout = DefaultTimeout
	}

	// Validate correlation_field if specified
	if rule.CorrelationField > 0 {
		if rule.compiledTriggerPattern.NumSubexp() < rule.CorrelationField {
			return fmt.Errorf("trigger_pattern has only %d capture groups, but correlation_field is %d",
				rule.compiledTriggerPattern.NumSubexp(), rule.CorrelationField)
		}
		if rule.compiledExpectedPattern.NumSubexp() < rule.CorrelationField {
			return fmt.Errorf("expected_pattern has only %d capture groups, but correlation_field is %d",
				rule.compiledExpectedPattern.NumSubexp(), rule.CorrelationField)
		}
	}

	return nil
}

func validateWebhook(wh *WebhookConfig) error {
	if wh.URL == "" {
		return errors.New("url is required")
	}

	// Validate URL format
	u, err := url.Parse(wh.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https, got %q", u.Scheme)
	}

	if u.Host == "" {
		return errors.New("url must have a host")
	}

	// Expand environment variables in token
	wh.Token = expandEnvVar(wh.Token)

	// Validate trigger if specified
	if wh.Trigger != "" {
		switch wh.Trigger {
		case WebhookTriggerOnIssues, WebhookTriggerAlways, WebhookTriggerNever:
			// Valid
		default:
			return fmt.Errorf("invalid trigger %q (must be on_issues, always, or never)", wh.Trigger)
		}
	} else {
		// Default to on_issues
		wh.Trigger = WebhookTriggerOnIssues
	}

	// Default timeout
	if wh.Timeout <= 0 {
		wh.Timeout = DefaultWebhookTimeout
	}

	return nil
}

// expandEnvVar expands environment variables in the format ${VAR} or $VAR.
func expandEnvVar(s string) string {
	if s == "" {
		return s
	}

	// Handle ${VAR} format
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		varName := s[2 : len(s)-1]
		return os.Getenv(varName)
	}

	// Handle $VAR format (no braces)
	if strings.HasPrefix(s, "$") && !strings.HasPrefix(s, "${") {
		varName := s[1:]
		return os.Getenv(varName)
	}

	return s
}
