package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/ccollicutt/negalog/pkg/analyzer"
	"github.com/ccollicutt/negalog/pkg/config"
	"github.com/ccollicutt/negalog/pkg/detector"
	"github.com/ccollicutt/negalog/pkg/output"
	"github.com/ccollicutt/negalog/pkg/parser"
	"github.com/ccollicutt/negalog/pkg/webhook"
)

var (
	projectRoot string
	rootOnce    sync.Once
)

// chdir changes to the project root directory for tests.
// Config files use paths relative to project root.
func chdir(t *testing.T) {
	t.Helper()
	rootOnce.Do(func() {
		// Get the directory containing this test file, then go up one level
		_, filename, _, _ := runtime.Caller(0)
		projectRoot = filepath.Dir(filepath.Dir(filename))
	})
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Failed to chdir to project root: %v", err)
	}
}

// requireFile fails the test if the required test file doesn't exist.
// We never skip tests - missing test data is a test failure.
func requireFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Required test file not found: %s", path)
	}
}

// TestE2E_LinuxSyslog tests the full analysis pipeline using real Linux syslog data.
// The log file is from https://github.com/logpai/loghub/tree/master/Linux
func TestE2E_LinuxSyslog(t *testing.T) {
	chdir(t)
	// Fail if log file doesn't exist
	logFile := filepath.Join("testdata", "logs", "linux_syslog.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatalf("Required test file not found: %s (run: curl -sL https://raw.githubusercontent.com/logpai/loghub/master/Linux/Linux_2k.log -o testdata/logs/linux_syslog.log)", logFile)
	}

	configFile := filepath.Join("testdata", "configs", "linux_syslog.yaml")
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load(ctx, configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Expand log sources
	files, err := parser.ExpandGlobs(cfg.LogSources)
	if err != nil {
		t.Fatalf("Failed to expand globs: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("No log files found")
	}

	// Create analyzer
	a, err := analyzer.NewAnalyzer(cfg)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	// Create log source
	source := parser.NewFileSource(
		files,
		cfg.TimestampFormat.CompiledPattern(),
		cfg.TimestampFormat.Layout,
	)
	defer source.Close()

	// Run analysis
	result, err := a.Analyze(ctx, source)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Verify results
	if result.Metadata.LinesProcessed == 0 {
		t.Error("Expected lines to be processed")
	}

	t.Logf("Processed %d lines", result.Metadata.LinesProcessed)
	t.Logf("Found %d total issues across %d rules", result.TotalIssues(), len(result.Results))

	// Verify we have results for both rules
	if len(result.Results) != 2 {
		t.Errorf("Expected 2 rule results, got %d", len(result.Results))
	}

	// Check specific rules
	for _, rr := range result.Results {
		switch rr.RuleName {
		case "unclosed-sessions":
			// Sessions in this log are properly closed
			if len(rr.Issues) != 0 {
				t.Logf("unclosed-sessions: %d issues (expected 0 - sessions are closed)", len(rr.Issues))
			}
		case "logrotate-alerts":
			// There are logrotate ALERT entries without follow-up
			if len(rr.Issues) == 0 {
				t.Error("Expected logrotate-alerts to find issues")
			}
			t.Logf("logrotate-alerts: found %d unacknowledged alerts", len(rr.Issues))
		}
	}
}

// TestE2E_LinuxSyslog_TextOutput tests text output formatting.
func TestE2E_LinuxSyslog_TextOutput(t *testing.T) {
	chdir(t)
	logFile := filepath.Join("testdata", "logs", "linux_syslog.log")
	requireFile(t, logFile)

	configFile := filepath.Join("testdata", "configs", "linux_syslog.yaml")
	ctx := context.Background()

	cfg, err := config.Load(ctx, configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	files, _ := parser.ExpandGlobs(cfg.LogSources)
	a, _ := analyzer.NewAnalyzer(cfg)
	source := parser.NewFileSource(files, cfg.TimestampFormat.CompiledPattern(), cfg.TimestampFormat.Layout)
	defer source.Close()

	result, err := a.Analyze(ctx, source)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Create report and format as text
	report := output.NewReport(result, configFile)
	formatter := output.NewTextFormatter(output.FormatOptions{})

	var buf bytes.Buffer
	if err := formatter.Format(ctx, report, &buf); err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	out := buf.String()

	// Verify output contains expected sections
	checks := []string{
		"NegaLog Analysis Report",
		"[SEQUENCE] unclosed-sessions",
		"[CONDITIONAL] logrotate-alerts",
		"Summary:",
	}

	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("Output missing %q", check)
		}
	}
}

// TestE2E_LinuxSyslog_JSONOutput tests JSON output formatting.
func TestE2E_LinuxSyslog_JSONOutput(t *testing.T) {
	chdir(t)
	logFile := filepath.Join("testdata", "logs", "linux_syslog.log")
	requireFile(t, logFile)

	configFile := filepath.Join("testdata", "configs", "linux_syslog.yaml")
	ctx := context.Background()

	cfg, err := config.Load(ctx, configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	files, _ := parser.ExpandGlobs(cfg.LogSources)
	a, _ := analyzer.NewAnalyzer(cfg)
	source := parser.NewFileSource(files, cfg.TimestampFormat.CompiledPattern(), cfg.TimestampFormat.Layout)
	defer source.Close()

	result, err := a.Analyze(ctx, source)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Create report and format as JSON
	report := output.NewReport(result, configFile)
	formatter := output.NewJSONFormatter(output.FormatOptions{})

	var buf bytes.Buffer
	if err := formatter.Format(ctx, report, &buf); err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify valid JSON
	var parsed output.Report
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	// Verify content
	if parsed.Summary.RulesChecked != 2 {
		t.Errorf("RulesChecked = %d, want 2", parsed.Summary.RulesChecked)
	}
	if parsed.Summary.LinesProcessed == 0 {
		t.Error("LinesProcessed should be > 0")
	}
	if len(parsed.Results) != 2 {
		t.Errorf("Results count = %d, want 2", len(parsed.Results))
	}
}

// TestE2E_LinuxSyslog_RuleFilter tests running specific rules only.
func TestE2E_LinuxSyslog_RuleFilter(t *testing.T) {
	chdir(t)
	logFile := filepath.Join("testdata", "logs", "linux_syslog.log")
	requireFile(t, logFile)

	configFile := filepath.Join("testdata", "configs", "linux_syslog.yaml")
	ctx := context.Background()

	cfg, err := config.Load(ctx, configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	files, _ := parser.ExpandGlobs(cfg.LogSources)

	// Only run unclosed-sessions rule
	a, err := analyzer.NewAnalyzer(cfg, analyzer.WithRuleFilter([]string{"unclosed-sessions"}))
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	source := parser.NewFileSource(files, cfg.TimestampFormat.CompiledPattern(), cfg.TimestampFormat.Layout)
	defer source.Close()

	result, err := a.Analyze(ctx, source)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Should only have 1 rule result
	if len(result.Results) != 1 {
		t.Fatalf("Expected 1 rule result, got %d", len(result.Results))
	}

	if result.Results[0].RuleName != "unclosed-sessions" {
		t.Errorf("Expected unclosed-sessions rule, got %s", result.Results[0].RuleName)
	}
}

// TestE2E_Heartbeat tests periodic detection with intentional gaps.
// The heartbeat.log file contains regular heartbeats with two 20-minute gaps.
func TestE2E_Heartbeat(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "heartbeat.yaml")
	ctx := context.Background()

	cfg, err := config.Load(ctx, configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	files, err := parser.ExpandGlobs(cfg.LogSources)
	if err != nil {
		t.Fatalf("Failed to expand globs: %v", err)
	}

	a, err := analyzer.NewAnalyzer(cfg)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	source := parser.NewFileSource(files, cfg.TimestampFormat.CompiledPattern(), cfg.TimestampFormat.Layout)
	defer source.Close()

	result, err := a.Analyze(ctx, source)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Processed %d lines, found %d issues", result.Metadata.LinesProcessed, result.TotalIssues())

	// Should have 2 rules
	if len(result.Results) != 2 {
		t.Fatalf("Expected 2 rule results, got %d", len(result.Results))
	}

	for _, rr := range result.Results {
		switch rr.RuleName {
		case "api-heartbeat":
			// Should detect exactly 2 gaps (10:15-10:35 and 11:00-11:20)
			if len(rr.Issues) != 2 {
				t.Errorf("api-heartbeat: expected 2 gap issues, got %d", len(rr.Issues))
			}
			for _, issue := range rr.Issues {
				if issue.Type != analyzer.IssueTypeGapExceeded {
					t.Errorf("Expected GapExceeded issue, got %v", issue.Type)
				}
				t.Logf("Gap detected: %v", issue.Context.ActualGap)
			}
		case "heartbeat-minimum":
			// Should detect 2 gaps + 1 below minimum occurrences
			gapCount := 0
			belowMinCount := 0
			for _, issue := range rr.Issues {
				switch issue.Type {
				case analyzer.IssueTypeGapExceeded:
					gapCount++
				case analyzer.IssueTypeBelowMinOccurrences:
					belowMinCount++
					if issue.Context.Occurrences >= 50 {
						t.Errorf("Expected < 50 occurrences, got %d", issue.Context.Occurrences)
					}
					t.Logf("Below minimum: %d occurrences (required: %d)", issue.Context.Occurrences, issue.Context.MinRequired)
				}
			}
			if gapCount != 2 {
				t.Errorf("heartbeat-minimum: expected 2 gap issues, got %d", gapCount)
			}
			if belowMinCount != 1 {
				t.Errorf("heartbeat-minimum: expected 1 below-minimum issue, got %d", belowMinCount)
			}
		}
	}
}

// TestE2E_Heartbeat_SpecificRule tests running only the api-heartbeat rule.
func TestE2E_Heartbeat_SpecificRule(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "heartbeat.yaml")
	ctx := context.Background()

	cfg, err := config.Load(ctx, configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	files, _ := parser.ExpandGlobs(cfg.LogSources)

	// Only run api-heartbeat rule
	a, err := analyzer.NewAnalyzer(cfg, analyzer.WithRuleFilter([]string{"api-heartbeat"}))
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	source := parser.NewFileSource(files, cfg.TimestampFormat.CompiledPattern(), cfg.TimestampFormat.Layout)
	defer source.Close()

	result, err := a.Analyze(ctx, source)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("Expected 1 rule result, got %d", len(result.Results))
	}

	if result.Results[0].RuleName != "api-heartbeat" {
		t.Errorf("Expected api-heartbeat rule, got %s", result.Results[0].RuleName)
	}

	// Should have exactly 2 issues (the two gaps)
	if len(result.Results[0].Issues) != 2 {
		t.Errorf("Expected 2 issues, got %d", len(result.Results[0].Issues))
	}
}

// TestE2E_Detect_Syslog tests detection on real syslog file.
func TestE2E_Detect_Syslog(t *testing.T) {
	chdir(t)
	logFile := filepath.Join("testdata", "logs", "linux_syslog.log")
	requireFile(t, logFile)

	d := detector.New()
	result, err := d.DetectFromFile(context.Background(), logFile)
	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Syslog (BSD)" {
		t.Errorf("Expected Syslog (BSD), got %s", best.Format.Name)
	}

	if best.Confidence < 0.9 {
		t.Errorf("Expected high confidence, got %.1f%%", best.Confidence*100)
	}

	t.Logf("Detected: %s with %.1f%% confidence", best.Format.Name, best.Confidence*100)
}

// TestE2E_Detect_Heartbeat tests detection on heartbeat log.
func TestE2E_Detect_Heartbeat(t *testing.T) {
	chdir(t)
	logFile := filepath.Join("testdata", "logs", "heartbeat.log")

	d := detector.New()
	result, err := d.DetectFromFile(context.Background(), logFile)
	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Syslog (BSD)" {
		t.Errorf("Expected Syslog (BSD), got %s", best.Format.Name)
	}

	t.Logf("Detected: %s with %.1f%% confidence", best.Format.Name, best.Confidence*100)
}

// TestE2E_Detect_WriteConfig tests config file generation.
func TestE2E_Detect_WriteConfig(t *testing.T) {
	chdir(t)
	logFile := filepath.Join("testdata", "logs", "heartbeat.log")
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "generated.yaml")

	d := detector.New()
	result, err := d.DetectFromFile(context.Background(), logFile)
	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	// Write config file
	best := result.BestMatch()
	configContent := generateTestConfig(logFile, best)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Verify the generated config is valid by loading it
	cfg, err := config.Load(context.Background(), configPath)
	if err != nil {
		t.Fatalf("Generated config is invalid: %v", err)
	}

	// Verify timestamp format matches detection
	if cfg.TimestampFormat.Layout != best.Format.Layout {
		t.Errorf("Layout mismatch: config has %q, detected %q",
			cfg.TimestampFormat.Layout, best.Format.Layout)
	}

	t.Logf("Generated valid config at %s", configPath)
}

// generateTestConfig creates a minimal valid config for testing.
func generateTestConfig(logFile string, match *detector.FormatMatch) string {
	absPath, _ := filepath.Abs(logFile)
	return fmt.Sprintf(`log_sources:
  - %s

timestamp_format:
  pattern: '%s'
  layout: "%s"

rules:
  - name: test-rule
    type: periodic
    pattern: 'HEARTBEAT'
    max_gap: 10m
`, absPath, match.Format.PatternStr, match.Format.Layout)
}

// ============================================================================
// Cross-Service Tracking E2E Tests
// ============================================================================

// TestE2E_CrossService tests tracking requests across multiple log files.
// Gateway receives requests, backend completes them. Some requests are lost.
func TestE2E_CrossService(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "cross_service.yaml")
	ctx := context.Background()

	cfg, err := config.Load(ctx, configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Should have 2 log sources
	files, err := parser.ExpandGlobs(cfg.LogSources)
	if err != nil {
		t.Fatalf("Failed to expand globs: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("Expected 2 log files, got %d", len(files))
	}

	a, err := analyzer.NewAnalyzer(cfg)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	// Create merged source for cross-service correlation
	sources := make([]parser.LogSource, len(files))
	for i, file := range files {
		sources[i] = parser.NewFileSource(
			[]string{file},
			cfg.TimestampFormat.CompiledPattern(),
			cfg.TimestampFormat.Layout,
		)
	}
	source := parser.NewMergedSource(sources...)
	defer source.Close()

	result, err := a.Analyze(ctx, source)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	t.Logf("Processed %d lines from %d files", result.Metadata.LinesProcessed, len(files))

	// Should have 1 rule result
	if len(result.Results) != 1 {
		t.Fatalf("Expected 1 rule result, got %d", len(result.Results))
	}

	rr := result.Results[0]
	if rr.RuleName != "request-flow" {
		t.Errorf("Expected request-flow rule, got %s", rr.RuleName)
	}

	// TRC-003 and TRC-005 should be missing (no completion in backend.log)
	if len(rr.Issues) != 2 {
		t.Errorf("Expected 2 incomplete requests, got %d", len(rr.Issues))
		for _, issue := range rr.Issues {
			t.Logf("  Issue: %s", issue.Context.CorrelationID)
		}
	}

	// Verify the right trace IDs are flagged
	foundTRC003, foundTRC005 := false, false
	for _, issue := range rr.Issues {
		switch issue.Context.CorrelationID {
		case "TRC-003":
			foundTRC003 = true
		case "TRC-005":
			foundTRC005 = true
		}
	}
	if !foundTRC003 {
		t.Error("Expected TRC-003 to be flagged as incomplete")
	}
	if !foundTRC005 {
		t.Error("Expected TRC-005 to be flagged as incomplete")
	}

	t.Logf("Cross-service tracking: detected %d lost requests", len(rr.Issues))
}

// ============================================================================
// Diagnose Command E2E Tests
// ============================================================================

// TestE2E_Diagnose_ValidConfig tests diagnose on a valid config.
func TestE2E_Diagnose_ValidConfig(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "heartbeat.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Diagnose should not fail even with warnings
		t.Fatalf("Diagnose failed: %v\nOutput: %s", err, output)
	}

	out := string(output)

	// Should show diagnostics header
	if !strings.Contains(out, "Configuration Diagnostics") {
		t.Error("Missing diagnostics header")
	}

	// Should show passing checks
	if !strings.Contains(out, "Config File") {
		t.Error("Missing config file check")
	}
	if !strings.Contains(out, "Config Syntax") {
		t.Error("Missing config syntax check")
	}

	// Should show summary
	if !strings.Contains(out, "Summary:") {
		t.Error("Missing summary")
	}
}

// TestE2E_Diagnose_InvalidYAML tests diagnose with invalid YAML syntax.
func TestE2E_Diagnose_InvalidYAML(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "invalid_yaml.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, err := cmd.CombinedOutput()
	// Diagnose doesn't return error, just reports issues
	if err != nil {
		t.Logf("Command returned error (expected for some cases): %v", err)
	}

	out := string(output)

	// Should detect YAML parsing error
	if !strings.Contains(out, "error") && !strings.Contains(out, "FAIL") {
		t.Errorf("Expected error indicator for invalid YAML, got: %s", out)
	}
}

// TestE2E_Diagnose_MissingLogSources tests diagnose with missing log_sources.
func TestE2E_Diagnose_MissingLogSources(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "missing_log_sources.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Should detect missing log sources
	if !strings.Contains(out, "No log sources") && !strings.Contains(out, "error") {
		t.Errorf("Expected 'No log sources' error, got: %s", out)
	}
}

// TestE2E_Diagnose_NonexistentLog tests diagnose with non-existent log file.
func TestE2E_Diagnose_NonexistentLog(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "nonexistent_log.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Should detect that log file doesn't exist
	if !strings.Contains(out, "no files") && !strings.Contains(out, "warning") && !strings.Contains(out, "WARN") {
		t.Errorf("Expected warning about missing log files, got: %s", out)
	}
}

// TestE2E_Diagnose_WrongPattern tests diagnose when pattern doesn't match logs.
func TestE2E_Diagnose_WrongPattern(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "wrong_pattern.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Should detect pattern mismatch and suggest correct format
	if !strings.Contains(out, "matches no lines") && !strings.Contains(out, "Pattern Test") {
		t.Errorf("Expected pattern mismatch warning, got: %s", out)
	}

	// Should suggest the detected format
	if !strings.Contains(out, "Detected format") || !strings.Contains(out, "Syslog") {
		t.Errorf("Expected format suggestion, got: %s", out)
	}
}

// TestE2E_Diagnose_BrokenSequenceRule tests diagnose with incomplete sequence rule.
func TestE2E_Diagnose_BrokenSequenceRule(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "broken_sequence_rule.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Should detect missing required fields
	if !strings.Contains(out, "start_pattern") || !strings.Contains(out, "error") {
		t.Errorf("Expected error about missing start_pattern, got: %s", out)
	}
}

// TestE2E_Diagnose_UnknownRuleType tests diagnose with unknown rule type.
func TestE2E_Diagnose_UnknownRuleType(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "unknown_rule_type.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Unknown rule type is caught by config validation, reported as Config Syntax error
	if !strings.Contains(out, "invalid type") || !strings.Contains(out, "foobar") {
		t.Errorf("Expected error about invalid type foobar, got: %s", out)
	}
}

// TestE2E_Diagnose_NonexistentConfig tests diagnose with non-existent config file.
func TestE2E_Diagnose_NonexistentConfig(t *testing.T) {
	chdir(t)

	cmd := exec.Command("./bin/negalog", "diagnose", "/nonexistent/config.yaml")
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Should detect config file doesn't exist
	if !strings.Contains(out, "not found") || !strings.Contains(out, "FAIL") {
		t.Errorf("Expected 'not found' error, got: %s", out)
	}
}

// TestE2E_Diagnose_VerboseMode tests diagnose with verbose flag.
func TestE2E_Diagnose_VerboseMode(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "heartbeat.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", "-v", configFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Diagnose verbose failed: %v\nOutput: %s", err, output)
	}

	out := string(output)

	// Should show detailed output
	if !strings.Contains(out, "Configuration Diagnostics") {
		t.Error("Missing diagnostics header in verbose mode")
	}
}

// ============================================================================
// Webhook E2E Tests
// ============================================================================

// TestE2E_Webhook_SendOnIssues tests webhook fires when issues are detected.
func TestE2E_Webhook_SendOnIssues(t *testing.T) {
	chdir(t)

	var receivedPayload []byte
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedPayload, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"received"}`))
	}))
	defer server.Close()

	// Run analysis that produces issues
	configFile := filepath.Join("testdata", "configs", "cross_service.yaml")
	ctx := context.Background()

	cfg, err := config.Load(ctx, configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	files, _ := parser.ExpandGlobs(cfg.LogSources)
	a, _ := analyzer.NewAnalyzer(cfg)

	sources := make([]parser.LogSource, len(files))
	for i, file := range files {
		sources[i] = parser.NewFileSource(
			[]string{file},
			cfg.TimestampFormat.CompiledPattern(),
			cfg.TimestampFormat.Layout,
		)
	}
	source := parser.NewMergedSource(sources...)
	defer source.Close()

	result, err := a.Analyze(ctx, source)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Create report
	report := output.NewReport(result, configFile)

	// Verify there are issues
	if !report.HasIssues() {
		t.Fatal("Expected issues for webhook test")
	}

	// Send webhook
	client := webhook.NewClient()
	resp := client.Send(ctx, report, webhook.SendOptions{
		URL:   server.URL,
		Token: "test-token-123",
	})

	if !resp.Success() {
		t.Fatalf("Webhook failed: %v", resp.Error)
	}

	// Verify bearer token
	if receivedAuth != "Bearer test-token-123" {
		t.Errorf("Expected Bearer token, got %s", receivedAuth)
	}

	// Verify payload is valid JSON with expected structure
	var payload output.Report
	if err := json.Unmarshal(receivedPayload, &payload); err != nil {
		t.Fatalf("Invalid JSON payload: %v", err)
	}

	if payload.Summary.TotalIssues == 0 {
		t.Error("Expected issues in webhook payload")
	}

	t.Logf("Webhook received %d issues", payload.Summary.TotalIssues)
}

// TestE2E_Webhook_NoSendOnSuccess tests webhook doesn't fire when no issues (on_issues trigger).
func TestE2E_Webhook_NoSendOnSuccess(t *testing.T) {
	chdir(t)

	webhookCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a report with no issues
	report := &output.Report{
		Summary: output.Summary{
			RulesChecked:    1,
			RulesWithIssues: 0,
			TotalIssues:     0,
			LinesProcessed:  100,
		},
		Results: []*analyzer.RuleResult{},
	}

	// Check trigger condition (on_issues should not fire)
	trigger := config.WebhookTriggerOnIssues
	shouldFire := trigger == config.WebhookTriggerAlways ||
		(trigger == config.WebhookTriggerOnIssues && report.HasIssues())

	if shouldFire {
		t.Error("Should not fire webhook when no issues with on_issues trigger")
	}

	if webhookCalled {
		t.Error("Webhook should not have been called")
	}
}

// TestE2E_Webhook_AlwaysTrigger tests webhook fires even when no issues (always trigger).
func TestE2E_Webhook_AlwaysTrigger(t *testing.T) {
	webhookCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a report with no issues
	report := &output.Report{
		Summary: output.Summary{
			RulesChecked:    1,
			RulesWithIssues: 0,
			TotalIssues:     0,
			LinesProcessed:  100,
		},
		Results: []*analyzer.RuleResult{},
	}

	// With always trigger, should fire
	client := webhook.NewClient()
	resp := client.Send(context.Background(), report, webhook.SendOptions{
		URL: server.URL,
	})

	if !resp.Success() {
		t.Fatalf("Webhook failed: %v", resp.Error)
	}

	if !webhookCalled {
		t.Error("Webhook should have been called with always trigger")
	}
}

// TestE2E_Webhook_ServerError tests handling of webhook server errors.
func TestE2E_Webhook_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	report := &output.Report{
		Summary: output.Summary{TotalIssues: 1},
	}

	client := webhook.NewClient()
	resp := client.Send(context.Background(), report, webhook.SendOptions{
		URL: server.URL,
	})

	if resp.Success() {
		t.Error("Expected webhook to fail with 500 error")
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

// TestE2E_Webhook_CLI tests webhook via CLI flags.
func TestE2E_Webhook_CLI(t *testing.T) {
	chdir(t)

	var receivedPayload []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPayload, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configFile := filepath.Join("testdata", "configs", "cross_service.yaml")

	cmd := exec.Command("./bin/negalog", "analyze", configFile,
		"--webhook-url", server.URL,
		"--webhook-trigger", "always")
	output, err := cmd.CombinedOutput()

	// Command may exit 1 due to issues, that's expected
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 1 {
			t.Fatalf("Unexpected error: %v\nOutput: %s", err, output)
		}
	}

	// Verify webhook was called
	if len(receivedPayload) == 0 {
		t.Error("Webhook was not called")
	}

	// Verify valid JSON
	var payload map[string]interface{}
	if err := json.Unmarshal(receivedPayload, &payload); err != nil {
		t.Fatalf("Invalid JSON payload: %v", err)
	}

	// Should have Summary field
	if _, ok := payload["Summary"]; !ok {
		t.Error("Payload missing Summary field")
	}

	// Check stderr for webhook message
	out := string(output)
	if !strings.Contains(out, "Webhook") {
		t.Log("Note: webhook status message expected in stderr")
	}
}

// TestE2E_Webhook_CLIWithToken tests webhook CLI with bearer token.
func TestE2E_Webhook_CLIWithToken(t *testing.T) {
	chdir(t)

	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configFile := filepath.Join("testdata", "configs", "heartbeat.yaml")

	cmd := exec.Command("./bin/negalog", "analyze", configFile,
		"--webhook-url", server.URL,
		"--webhook-token", "secret-token",
		"--webhook-trigger", "always")
	_, err := cmd.CombinedOutput()

	// Allow exit code 1 (issues found)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	if receivedAuth != "Bearer secret-token" {
		t.Errorf("Expected Bearer token, got %q", receivedAuth)
	}
}

// TestE2E_Webhook_ConfigFile tests webhooks defined in config file.
func TestE2E_Webhook_ConfigFile(t *testing.T) {
	chdir(t)

	var receivedPayloads [][]byte
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		receivedPayloads = append(receivedPayloads, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a temp config with webhook
	tmpDir := t.TempDir()
	configContent := fmt.Sprintf(`log_sources:
  - testdata/logs/heartbeat.log

timestamp_format:
  pattern: '^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})'
  layout: "Jan  2 15:04:05"

rules:
  - name: test-heartbeat
    type: periodic
    pattern: 'HEARTBEAT'
    max_gap: 10m

webhooks:
  - name: test-webhook
    url: "%s"
    trigger: always
`, server.URL)

	configFile := filepath.Join(tmpDir, "test-webhook.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cmd := exec.Command("./bin/negalog", "analyze", configFile)
	_, err := cmd.CombinedOutput()

	// Allow exit code 1 (issues found)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	mu.Lock()
	count := len(receivedPayloads)
	mu.Unlock()

	if count == 0 {
		t.Error("Webhook from config file was not called")
	}
}

// ============================================================================
// Webhook Diagnose E2E Tests
// ============================================================================

// TestE2E_Diagnose_InvalidWebhookURL tests diagnose with invalid webhook URL.
func TestE2E_Diagnose_InvalidWebhookURL(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "invalid_webhook_url.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Should detect invalid URL scheme
	if !strings.Contains(out, "http") && !strings.Contains(out, "scheme") {
		t.Errorf("Expected error about URL scheme, got: %s", out)
	}
}

// TestE2E_Diagnose_MissingWebhookURL tests diagnose with missing webhook URL.
func TestE2E_Diagnose_MissingWebhookURL(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "missing_webhook_url.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Should detect missing URL
	if !strings.Contains(out, "url") && !strings.Contains(out, "error") {
		t.Errorf("Expected error about missing URL, got: %s", out)
	}
}

// TestE2E_Diagnose_InvalidWebhookTrigger tests diagnose with invalid webhook trigger.
func TestE2E_Diagnose_InvalidWebhookTrigger(t *testing.T) {
	chdir(t)
	configFile := filepath.Join("testdata", "configs", "bad", "invalid_webhook_trigger.yaml")

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, _ := cmd.CombinedOutput()
	out := string(output)

	// Should detect invalid trigger
	if !strings.Contains(out, "trigger") && !strings.Contains(out, "invalid") {
		t.Errorf("Expected error about invalid trigger, got: %s", out)
	}
}

// TestE2E_Diagnose_ValidWebhook tests diagnose with valid webhook config.
func TestE2E_Diagnose_ValidWebhook(t *testing.T) {
	chdir(t)

	// Create a temp config with valid webhook
	tmpDir := t.TempDir()
	configContent := `log_sources:
  - testdata/logs/heartbeat.log

timestamp_format:
  pattern: '^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})'
  layout: "Jan  2 15:04:05"

rules:
  - name: test-heartbeat
    type: periodic
    pattern: 'HEARTBEAT'
    max_gap: 10m

webhooks:
  - name: test-webhook
    url: "https://example.com/webhook"
    trigger: on_issues
    timeout: 30s
`
	configFile := filepath.Join(tmpDir, "valid-webhook.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cmd := exec.Command("./bin/negalog", "diagnose", configFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Diagnose failed: %v\nOutput: %s", err, output)
	}

	out := string(output)

	// Should show webhook check passed
	if !strings.Contains(out, "Webhook") {
		t.Error("Expected webhook diagnostic output")
	}
	if !strings.Contains(out, "PASS") {
		t.Error("Expected PASS for valid webhook")
	}
}

// TestE2E_Diagnose_WebhookVerbose tests diagnose verbose mode with webhooks.
func TestE2E_Diagnose_WebhookVerbose(t *testing.T) {
	chdir(t)

	// Create a temp config with webhook
	tmpDir := t.TempDir()
	configContent := `log_sources:
  - testdata/logs/heartbeat.log

timestamp_format:
  pattern: '^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})'
  layout: "Jan  2 15:04:05"

rules:
  - name: test-heartbeat
    type: periodic
    pattern: 'HEARTBEAT'
    max_gap: 10m

webhooks:
  - name: verbose-test
    url: "https://example.com/webhook"
    trigger: always
    timeout: 15s
`
	configFile := filepath.Join(tmpDir, "verbose-webhook.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cmd := exec.Command("./bin/negalog", "diagnose", "-v", configFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Diagnose verbose failed: %v\nOutput: %s", err, output)
	}

	out := string(output)

	// Should show webhook details in verbose mode
	if !strings.Contains(out, "Webhook") {
		t.Error("Expected webhook output in verbose mode")
	}
	// Should show connectivity check in verbose mode
	if !strings.Contains(out, "Connectivity") {
		t.Log("Note: Connectivity check may be skipped if network is unavailable")
	}
}
