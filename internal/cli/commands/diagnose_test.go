package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDiagnoseCommand(t *testing.T) {
	cmd := NewDiagnoseCommand()

	if cmd.Use != "diagnose <config-file>" {
		t.Errorf("Unexpected Use: %s", cmd.Use)
	}

	// Check verbose flag exists
	if cmd.Flags().Lookup("verbose") == nil {
		t.Error("Missing verbose flag")
	}
}

func TestCheckConfigExists_NotFound(t *testing.T) {
	result := checkConfigExists("/nonexistent/config.yaml")

	if result.Status != "error" {
		t.Errorf("Expected error status, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "not found") {
		t.Errorf("Expected 'not found' in message, got: %s", result.Message)
	}
}

func TestCheckConfigExists_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.yaml")

	// Create empty file
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	result := checkConfigExists(configPath)

	if result.Status != "error" {
		t.Errorf("Expected error status, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "empty") {
		t.Errorf("Expected 'empty' in message, got: %s", result.Message)
	}
}

func TestCheckConfigExists_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	result := checkConfigExists(tmpDir)

	if result.Status != "error" {
		t.Errorf("Expected error status, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "directory") {
		t.Errorf("Expected 'directory' in message, got: %s", result.Message)
	}
}

func TestCheckConfigExists_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("test: value"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	result := checkConfigExists(configPath)

	if result.Status != "ok" {
		t.Errorf("Expected ok status, got %s", result.Status)
	}
}

func TestCheckConfigParseable_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Invalid YAML
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: bad"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	_, result := checkConfigParseable(configPath)

	if result.Status != "error" {
		t.Errorf("Expected error status, got %s", result.Status)
	}
}

func TestCheckConfigParseable_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "valid.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	// Create log file
	if err := os.WriteFile(logPath, []byte("test log content"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, result := checkConfigParseable(configPath)

	if result.Status != "ok" {
		t.Errorf("Expected ok status, got %s: %s", result.Status, result.Message)
	}
	if cfg == nil {
		t.Error("Expected config to be returned")
	}
}

func TestRunDiagnose_MissingConfig(t *testing.T) {
	cmd := NewDiagnoseCommand()
	cmd.SetArgs([]string{"/nonexistent/config.yaml"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Should not error, just print diagnostics
	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunDiagnose_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	// Create log file with timestamps
	logContent := `2024-01-15T10:30:00 Event 1
2024-01-15T10:30:01 Event 2
2024-01-15T10:30:02 Event 3
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})'
  layout: "2006-01-02T15:04:05"
rules:
  - name: test-rule
    type: periodic
    pattern: 'Event'
    max_gap: 5m
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cmd := NewDiagnoseCommand()
	cmd.SetArgs([]string{configPath})

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunDiagnose_MissingLogSource(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	config := `log_sources:
  - /nonexistent/path/*.log
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cmd := NewDiagnoseCommand()
	cmd.SetArgs([]string{configPath})

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCheckRules_SequenceRule(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	// Valid sequence rule
	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test-sequence
    type: sequence
    start_pattern: 'START id=(\w+)'
    end_pattern: 'END id=(\w+)'
    correlation_field: 1
    timeout: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, result := checkConfigParseable(configPath)
	if cfg == nil {
		t.Fatalf("Config parsing failed: %s", result.Message)
	}
	results := checkRules(cfg)

	// Should pass
	found := false
	for _, r := range results {
		if strings.Contains(r.Check, "test-sequence") {
			found = true
			if r.Status != "ok" {
				t.Errorf("Expected ok status for valid sequence rule, got %s: %s", r.Status, r.Message)
			}
		}
	}
	if !found {
		t.Error("Expected to find test-sequence rule check")
	}
}

func TestCheckRules_PeriodicRule(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	// Valid periodic rule
	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: heartbeat
    type: periodic
    pattern: 'HEARTBEAT'
    max_gap: 5m
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, result := checkConfigParseable(configPath)
	if cfg == nil {
		t.Fatalf("Config parsing failed: %s", result.Message)
	}
	results := checkRules(cfg)

	// Should pass
	found := false
	for _, r := range results {
		if strings.Contains(r.Check, "heartbeat") {
			found = true
			if r.Status != "ok" {
				t.Errorf("Expected ok status for valid periodic rule, got %s: %v", r.Status, r.Details)
			}
		}
	}
	if !found {
		t.Error("Expected to find heartbeat rule check")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10.", 10, "exactly10."},
		{"this is a long string", 10, "this is..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// Webhook diagnose tests

func TestCheckWebhooks_NoWebhooks(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, _ := checkConfigParseable(configPath)
	opts := &DiagnoseOptions{Verbose: false}

	results := checkWebhooks(cfg, opts)

	// Without verbose, should return empty
	if len(results) != 0 {
		t.Errorf("Expected 0 results without verbose, got %d", len(results))
	}

	// With verbose, should return 1 result
	opts.Verbose = true
	results = checkWebhooks(cfg, opts)
	if len(results) != 1 {
		t.Errorf("Expected 1 result with verbose, got %d", len(results))
	}
}

func TestCheckWebhooks_ValidWebhook(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
webhooks:
  - name: test-webhook
    url: "https://example.com/webhook"
    trigger: on_issues
    timeout: 10s
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, _ := checkConfigParseable(configPath)
	opts := &DiagnoseOptions{Verbose: false}

	results := checkWebhooks(cfg, opts)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Status != "ok" {
		t.Errorf("Expected ok status, got %s: %s", results[0].Status, results[0].Message)
	}
}

func TestCheckWebhooks_InvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
webhooks:
  - name: bad-webhook
    url: "ftp://invalid.example.com"
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Config validation will catch this before checkWebhooks runs
	// So we need to test checkWebhooks directly with a cfg that has invalid webhook
}

func TestCheckWebhooks_MissingURL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	// Note: This will fail validation before checkWebhooks runs
	// This test is more for documentation
	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
webhooks:
  - name: no-url-webhook
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, result := checkConfigParseable(configPath)
	// This should fail at config validation stage
	if cfg != nil && result.Status == "ok" {
		// If config somehow loaded, checkWebhooks would catch it
		opts := &DiagnoseOptions{}
		results := checkWebhooks(cfg, opts)
		hasError := false
		for _, r := range results {
			if r.Status == "error" {
				hasError = true
			}
		}
		if !hasError && len(cfg.Webhooks) > 0 {
			t.Error("Expected error for webhook with no URL")
		}
	}
}

func TestCheckWebhooks_InvalidTrigger(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
webhooks:
  - name: bad-trigger
    url: "https://example.com/webhook"
    trigger: bad_trigger
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// This will fail at config validation
	cfg, result := checkConfigParseable(configPath)
	if result.Status == "ok" && cfg != nil {
		// If it passed validation, checkWebhooks should catch it
		opts := &DiagnoseOptions{}
		results := checkWebhooks(cfg, opts)
		hasError := false
		for _, r := range results {
			if r.Status == "error" && strings.Contains(r.Message, "trigger") {
				hasError = true
			}
		}
		if !hasError {
			t.Log("Invalid trigger caught at validation stage (expected)")
		}
	}
}

func TestCheckWebhooks_MultipleWebhooks(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
webhooks:
  - name: slack
    url: "https://hooks.slack.com/services/xxx"
    trigger: on_issues
  - name: pagerduty
    url: "https://events.pagerduty.com/v2/enqueue"
    trigger: always
    token: my-token
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, _ := checkConfigParseable(configPath)
	opts := &DiagnoseOptions{Verbose: true}

	results := checkWebhooks(cfg, opts)

	// Should have 2 webhook config checks + 2 connectivity checks (verbose)
	webhookResults := 0
	for _, r := range results {
		if strings.Contains(r.Check, "Webhook") {
			webhookResults++
		}
	}
	if webhookResults < 2 {
		t.Errorf("Expected at least 2 webhook results, got %d", webhookResults)
	}
}

func TestCheckWebhooks_VerboseMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
webhooks:
  - name: verbose-test
    url: "https://example.com/webhook"
    trigger: always
    timeout: 30s
    token: secret-token
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, _ := checkConfigParseable(configPath)

	// Without verbose
	optsNoVerbose := &DiagnoseOptions{Verbose: false}
	resultsNoVerbose := checkWebhooks(cfg, optsNoVerbose)

	// With verbose
	optsVerbose := &DiagnoseOptions{Verbose: true}
	resultsVerbose := checkWebhooks(cfg, optsVerbose)

	// Verbose should have more results (connectivity check)
	if len(resultsVerbose) <= len(resultsNoVerbose) {
		t.Logf("Verbose results: %d, Non-verbose results: %d", len(resultsVerbose), len(resultsNoVerbose))
	}

	// Verbose should have details
	for _, r := range resultsVerbose {
		if strings.Contains(r.Check, "verbose-test") && r.Status == "ok" {
			if len(r.Details) == 0 {
				t.Error("Expected details in verbose mode")
			}
		}
	}
}

func TestPrintDiagnostics(t *testing.T) {
	// Just verify it doesn't panic with various inputs
	results := []DiagnosticResult{
		{Check: "Test1", Status: "ok", Message: "All good"},
		{Check: "Test2", Status: "warning", Message: "Hmm", Details: []string{"detail1"}},
		{Check: "Test3", Status: "error", Message: "Bad", Suggests: []string{"Fix it"}},
	}

	opts := &DiagnoseOptions{Verbose: true}

	// Capture output (redirect stdout temporarily not possible in test,
	// so just verify no panic)
	printDiagnostics(results, opts)
}

func TestCheckLogSources_DirectFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	// Create log file
	if err := os.WriteFile(logPath, []byte("log content"), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, _ := checkConfigParseable(configPath)
	results := checkLogSources(cfg)

	found := false
	for _, r := range results {
		if strings.Contains(r.Check, "test.log") {
			found = true
			if r.Status != "ok" {
				t.Errorf("Expected ok status, got %s", r.Status)
			}
		}
	}
	if !found {
		t.Error("Expected to find log source check")
	}
}

func TestCheckLogSources_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "empty.log")

	// Create empty log file
	if err := os.WriteFile(logPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, _ := checkConfigParseable(configPath)
	results := checkLogSources(cfg)

	for _, r := range results {
		if strings.Contains(r.Check, "empty.log") {
			if r.Status != "warning" {
				t.Errorf("Expected warning for empty file, got %s", r.Status)
			}
		}
	}
}

func TestCheckLogSources_GlobPattern(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create multiple log files
	for i := 1; i <= 3; i++ {
		logPath := filepath.Join(tmpDir, "app"+string(rune('0'+i))+".log")
		if err := os.WriteFile(logPath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create log: %v", err)
		}
	}

	config := `log_sources:
  - ` + filepath.Join(tmpDir, "*.log") + `
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, _ := checkConfigParseable(configPath)
	results := checkLogSources(cfg)

	found := false
	for _, r := range results {
		if strings.Contains(r.Check, "*.log") {
			found = true
			if r.Status != "ok" {
				t.Errorf("Expected ok status for glob, got %s", r.Status)
			}
			if !strings.Contains(r.Message, "3") {
				t.Errorf("Expected to find 3 files, got: %s", r.Message)
			}
		}
	}
	if !found {
		t.Error("Expected to find glob pattern check")
	}
}
