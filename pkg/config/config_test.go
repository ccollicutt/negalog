package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
log_sources:
  - /var/log/*.log
timestamp_format:
  pattern: '^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]'
  layout: "2006-01-02 15:04:05"
rules:
  - name: test-sequence
    type: sequence
    start_pattern: 'START id=(\w+)'
    end_pattern: 'END id=(\w+)'
    correlation_field: 1
    timeout: 30s
`
	path := writeTempFile(t, "config.yaml", content)
	cfg, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.LogSources) != 1 {
		t.Errorf("LogSources = %d, want 1", len(cfg.LogSources))
	}
	if len(cfg.Rules) != 1 {
		t.Errorf("Rules = %d, want 1", len(cfg.Rules))
	}
	if cfg.Rules[0].Name != "test-sequence" {
		t.Errorf("Rule name = %q, want %q", cfg.Rules[0].Name, "test-sequence")
	}
	if cfg.Rules[0].Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Rules[0].Timeout)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(context.Background(), "/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	content := `invalid: yaml: content: [`
	path := writeTempFile(t, "invalid.yaml", content)
	_, err := Load(context.Background(), path)
	if err == nil {
		t.Error("Load() expected error for invalid YAML")
	}
}

func TestValidate_NoLogSources(t *testing.T) {
	cfg := &Config{
		LogSources: []string{},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d+)\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{Name: "test", Type: "sequence"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for empty log_sources")
	}
}

func TestValidate_NoRules(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d+)\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for empty rules")
	}
}

func TestValidate_InvalidTimestampPattern(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `[invalid`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{Name: "test", Type: "sequence"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for invalid regex")
	}
}

func TestValidate_TimestampPatternNoCaptureGroup(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\d+`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{Name: "test", Type: "sequence"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for pattern without capture group")
	}
}

func TestValidate_SequenceRule_Valid(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:             "test",
			Type:             "sequence",
			StartPattern:     `START id=(\w+)`,
			EndPattern:       `END id=(\w+)`,
			CorrelationField: 1,
			Timeout:          30 * time.Second,
		}},
	}
	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestValidate_SequenceRule_MissingStartPattern(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:             "test",
			Type:             "sequence",
			EndPattern:       `END id=(\w+)`,
			CorrelationField: 1,
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for missing start_pattern")
	}
}

func TestValidate_SequenceRule_InvalidCorrelationField(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:             "test",
			Type:             "sequence",
			StartPattern:     `START id=(\w+)`,
			EndPattern:       `END id=(\w+)`,
			CorrelationField: 5, // Only 1 capture group
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for invalid correlation_field")
	}
}

func TestValidate_PeriodicRule_Valid(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
			MaxGap:  5 * time.Minute,
		}},
	}
	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestValidate_PeriodicRule_MissingPattern(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:   "test",
			Type:   "periodic",
			MaxGap: 5 * time.Minute,
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for missing pattern")
	}
}

func TestValidate_ConditionalRule_Valid(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:            "test",
			Type:            "conditional",
			TriggerPattern:  `ERROR`,
			ExpectedPattern: `ALERT`,
			Timeout:         10 * time.Second,
		}},
	}
	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestValidate_ConditionalRule_MissingTrigger(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:            "test",
			Type:            "conditional",
			ExpectedPattern: `ALERT`,
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for missing trigger_pattern")
	}
}

func TestValidate_InvalidRuleType(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name: "test",
			Type: "invalid",
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for invalid rule type")
	}
}

func TestValidate_DefaultTimeout(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:             "test",
			Type:             "sequence",
			StartPattern:     `START id=(\w+)`,
			EndPattern:       `END id=(\w+)`,
			CorrelationField: 1,
			// No timeout specified
		}},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Rules[0].Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want default %v", cfg.Rules[0].Timeout, DefaultTimeout)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.TimestampFormat.Pattern == "" {
		t.Error("DefaultConfig() has empty timestamp pattern")
	}
	if cfg.TimestampFormat.Layout == "" {
		t.Error("DefaultConfig() has empty timestamp layout")
	}
}

func TestCompiledPatterns(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:             "seq",
			Type:             "sequence",
			StartPattern:     `START id=(\w+)`,
			EndPattern:       `END id=(\w+)`,
			CorrelationField: 1,
		}, {
			Name:    "per",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}, {
			Name:            "cond",
			Type:            "conditional",
			TriggerPattern:  `ERROR`,
			ExpectedPattern: `ALERT`,
		}},
	}

	err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	// Check timestamp pattern compiled
	if cfg.TimestampFormat.CompiledPattern() == nil {
		t.Error("TimestampFormat.CompiledPattern() is nil")
	}

	// Check sequence patterns compiled
	if cfg.Rules[0].CompiledStartPattern() == nil {
		t.Error("CompiledStartPattern() is nil")
	}
	if cfg.Rules[0].CompiledEndPattern() == nil {
		t.Error("CompiledEndPattern() is nil")
	}

	// Check periodic pattern compiled
	if cfg.Rules[1].CompiledPattern() == nil {
		t.Error("CompiledPattern() is nil")
	}

	// Check conditional patterns compiled
	if cfg.Rules[2].CompiledTriggerPattern() == nil {
		t.Error("CompiledTriggerPattern() is nil")
	}
	if cfg.Rules[2].CompiledExpectedPattern() == nil {
		t.Error("CompiledExpectedPattern() is nil")
	}
}

func TestRuleTypeEnum(t *testing.T) {
	tests := []struct {
		input string
		want  RuleType
	}{
		{"sequence", RuleTypeSequence},
		{"periodic", RuleTypePeriodic},
		{"conditional", RuleTypeConditional},
	}

	for _, tt := range tests {
		r := RuleConfig{Type: tt.input}
		if got := r.RuleTypeEnum(); got != tt.want {
			t.Errorf("RuleTypeEnum(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ============================================================================
// Webhook Validation Tests
// ============================================================================

func TestValidate_Webhook_Valid(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}},
		Webhooks: []WebhookConfig{{
			Name:    "test-webhook",
			URL:     "https://example.com/webhook",
			Trigger: WebhookTriggerOnIssues,
			Timeout: 10 * time.Second,
		}},
	}
	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestValidate_Webhook_ValidHTTP(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}},
		Webhooks: []WebhookConfig{{
			URL: "http://localhost:8080/webhook",
		}},
	}
	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestValidate_Webhook_MissingURL(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}},
		Webhooks: []WebhookConfig{{
			Name:    "no-url",
			Trigger: WebhookTriggerOnIssues,
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for missing URL")
	}
}

func TestValidate_Webhook_InvalidScheme(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}},
		Webhooks: []WebhookConfig{{
			URL: "ftp://example.com/webhook",
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for non-http scheme")
	}
}

func TestValidate_Webhook_InvalidTrigger(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}},
		Webhooks: []WebhookConfig{{
			URL:     "https://example.com/webhook",
			Trigger: "invalid_trigger",
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() expected error for invalid trigger")
	}
}

func TestValidate_Webhook_AllTriggers(t *testing.T) {
	triggers := []WebhookTrigger{
		WebhookTriggerOnIssues,
		WebhookTriggerAlways,
		WebhookTriggerNever,
	}

	for _, trigger := range triggers {
		cfg := &Config{
			LogSources: []string{"/var/log/*.log"},
			TimestampFormat: TimestampConfig{
				Pattern: `^\[(\d{4})\]`,
				Layout:  "2006",
			},
			Rules: []RuleConfig{{
				Name:    "test",
				Type:    "periodic",
				Pattern: `HEARTBEAT`,
			}},
			Webhooks: []WebhookConfig{{
				URL:     "https://example.com/webhook",
				Trigger: trigger,
			}},
		}
		err := Validate(cfg)
		if err != nil {
			t.Errorf("Validate() with trigger %q error = %v", trigger, err)
		}
	}
}

func TestValidate_Webhook_DefaultTrigger(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}},
		Webhooks: []WebhookConfig{{
			URL: "https://example.com/webhook",
			// No trigger specified
		}},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Webhooks[0].Trigger != WebhookTriggerOnIssues {
		t.Errorf("Default trigger = %v, want %v", cfg.Webhooks[0].Trigger, WebhookTriggerOnIssues)
	}
}

func TestValidate_Webhook_DefaultTimeout(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}},
		Webhooks: []WebhookConfig{{
			URL: "https://example.com/webhook",
			// No timeout specified
		}},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Webhooks[0].Timeout != DefaultWebhookTimeout {
		t.Errorf("Default timeout = %v, want %v", cfg.Webhooks[0].Timeout, DefaultWebhookTimeout)
	}
}

func TestValidate_Webhook_MultipleWebhooks(t *testing.T) {
	cfg := &Config{
		LogSources: []string{"/var/log/*.log"},
		TimestampFormat: TimestampConfig{
			Pattern: `^\[(\d{4})\]`,
			Layout:  "2006",
		},
		Rules: []RuleConfig{{
			Name:    "test",
			Type:    "periodic",
			Pattern: `HEARTBEAT`,
		}},
		Webhooks: []WebhookConfig{
			{
				Name:    "slack",
				URL:     "https://hooks.slack.com/services/xxx",
				Trigger: WebhookTriggerOnIssues,
			},
			{
				Name:    "pagerduty",
				URL:     "https://events.pagerduty.com/v2/enqueue",
				Token:   "test-token",
				Trigger: WebhookTriggerAlways,
			},
		},
	}
	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestExpandEnvVar(t *testing.T) {
	// Set test env var
	os.Setenv("TEST_WEBHOOK_TOKEN", "secret-value")
	defer os.Unsetenv("TEST_WEBHOOK_TOKEN")

	tests := []struct {
		input string
		want  string
	}{
		{"${TEST_WEBHOOK_TOKEN}", "secret-value"},
		{"$TEST_WEBHOOK_TOKEN", "secret-value"},
		{"plain-value", "plain-value"},
		{"", ""},
		{"${NONEXISTENT_VAR}", ""},
	}

	for _, tt := range tests {
		got := expandEnvVar(tt.input)
		if got != tt.want {
			t.Errorf("expandEnvVar(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoad_WithWebhooks(t *testing.T) {
	content := `
log_sources:
  - /var/log/*.log
timestamp_format:
  pattern: '^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]'
  layout: "2006-01-02 15:04:05"
rules:
  - name: test-rule
    type: periodic
    pattern: 'HEARTBEAT'
    max_gap: 5m
webhooks:
  - name: test-webhook
    url: "https://example.com/webhook"
    trigger: on_issues
    timeout: 30s
  - url: "https://backup.example.com/webhook"
    trigger: always
`
	path := writeTempFile(t, "config-with-webhooks.yaml", content)
	cfg, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Webhooks) != 2 {
		t.Errorf("Webhooks = %d, want 2", len(cfg.Webhooks))
	}
	if cfg.Webhooks[0].Name != "test-webhook" {
		t.Errorf("Webhook[0].Name = %q, want %q", cfg.Webhooks[0].Name, "test-webhook")
	}
	if cfg.Webhooks[0].Trigger != WebhookTriggerOnIssues {
		t.Errorf("Webhook[0].Trigger = %v, want %v", cfg.Webhooks[0].Trigger, WebhookTriggerOnIssues)
	}
	if cfg.Webhooks[0].Timeout != 30*time.Second {
		t.Errorf("Webhook[0].Timeout = %v, want 30s", cfg.Webhooks[0].Timeout)
	}
	if cfg.Webhooks[1].Trigger != WebhookTriggerAlways {
		t.Errorf("Webhook[1].Trigger = %v, want %v", cfg.Webhooks[1].Trigger, WebhookTriggerAlways)
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	return path
}
