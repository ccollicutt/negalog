package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"negalog/pkg/detector"
)

func TestNewAnalyzeCommand(t *testing.T) {
	cmd := NewAnalyzeCommand()

	if cmd.Use != "analyze <config-file>" {
		t.Errorf("Unexpected Use: %s", cmd.Use)
	}

	// Check flags exist
	flags := []string{"output", "time-range", "rule", "verbose", "quiet"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("Missing flag: %s", flag)
		}
	}
}

func TestNewValidateCommand(t *testing.T) {
	cmd := NewValidateCommand()

	if cmd.Use != "validate <config-file>" {
		t.Errorf("Unexpected Use: %s", cmd.Use)
	}

	if !strings.Contains(cmd.Long, "Validate") {
		t.Error("Missing description in Long")
	}
}

func TestNewVersionCommand(t *testing.T) {
	cmd := NewVersionCommand()

	if cmd.Use != "version" {
		t.Errorf("Unexpected Use: %s", cmd.Use)
	}
}

func TestRunValidate_Success(t *testing.T) {
	// Create a valid config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	logPath := filepath.Join(tmpDir, "test.log")

	// Create log file
	if err := os.WriteFile(logPath, []byte("test log"), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	config := `log_sources:
  - ` + logPath + `

timestamp_format:
  pattern: '^(\d{4}-\d{2}-\d{2})'
  layout: "2006-01-02"

rules:
  - name: test
    type: periodic
    pattern: 'test'
    max_gap: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cmd := NewValidateCommand()
	cmd.SetArgs([]string{configPath})

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestRunValidate_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Invalid YAML
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content"), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cmd := NewValidateCommand()
	cmd.SetArgs([]string{configPath})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestRunValidate_MissingFile(t *testing.T) {
	cmd := NewValidateCommand()
	cmd.SetArgs([]string{"/nonexistent/config.yaml"})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestRunAnalyze_MissingFile(t *testing.T) {
	cmd := NewAnalyzeCommand()
	cmd.SetArgs([]string{"/nonexistent/config.yaml"})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestRunAnalyze_InvalidTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	config := `log_sources:
  - /tmp/*.log
timestamp_format:
  pattern: '^(\d{4})'
  layout: "2006"
rules:
  - name: test
    type: periodic
    pattern: 'x'
    max_gap: 1h
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cmd := NewAnalyzeCommand()
	cmd.SetArgs([]string{"--time-range", "invalid", configPath})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Error("Expected error for invalid time-range")
	}
	if !strings.Contains(err.Error(), "invalid time-range") {
		t.Errorf("Expected 'invalid time-range' error, got: %v", err)
	}
}

func TestCreateFormatter(t *testing.T) {
	tests := []struct {
		output  string
		wantErr bool
	}{
		{"text", false},
		{"json", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			opts := &AnalyzeOptions{Output: tt.output}
			_, err := createFormatter(opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("createFormatter(%q) error = %v, wantErr %v", tt.output, err, tt.wantErr)
			}
		})
	}
}

func TestOutputDetectText_NoMatch(t *testing.T) {
	result := &detector.DetectionResult{
		Matches:      []detector.FormatMatch{},
		SampledLines: 100,
		ParsedLines:  0,
	}
	opts := &DetectOptions{}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputDetectText(result, "/test/file.log", opts)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(output, "No timestamp format detected") {
		t.Error("Expected 'No timestamp format detected' message")
	}
}

func TestOutputDetectText_WithMatch(t *testing.T) {
	format := &detector.TimestampFormat{
		Name:       "Test Format",
		PatternStr: "^test",
		Layout:     "2006",
	}
	result := &detector.DetectionResult{
		Matches: []detector.FormatMatch{
			{
				Format:     format,
				Confidence: 0.95,
				MatchCount: 95,
				SampleLine: "test line",
			},
		},
		SampledLines: 100,
		ParsedLines:  95,
	}
	opts := &DetectOptions{}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputDetectText(result, "/test/file.log", opts)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(output, "Test Format") {
		t.Error("Expected format name in output")
	}
	if !strings.Contains(output, "95.0%") {
		t.Error("Expected confidence in output")
	}
}

func TestOutputDetectText_Ambiguous(t *testing.T) {
	format := &detector.TimestampFormat{
		Name:       "Ambiguous Format",
		PatternStr: "^test",
		Layout:     "2006",
		Ambiguous:  true,
	}
	result := &detector.DetectionResult{
		Matches: []detector.FormatMatch{
			{Format: format, Confidence: 1.0, MatchCount: 100},
		},
		SampledLines:  100,
		ParsedLines:   100,
		AmbiguityNote: "Test ambiguity note",
	}
	opts := &DetectOptions{}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputDetectText(result, "/test/file.log", opts)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(output, "WARNING") {
		t.Error("Expected WARNING for ambiguous format")
	}
	if !strings.Contains(output, "Test ambiguity note") {
		t.Error("Expected ambiguity note in output")
	}
}

func TestOutputDetectText_ShowAll(t *testing.T) {
	format1 := &detector.TimestampFormat{Name: "Format 1", PatternStr: "^a", Layout: "2006"}
	format2 := &detector.TimestampFormat{Name: "Format 2", PatternStr: "^b", Layout: "2006"}
	result := &detector.DetectionResult{
		Matches: []detector.FormatMatch{
			{Format: format1, Confidence: 0.9, MatchCount: 90},
			{Format: format2, Confidence: 0.5, MatchCount: 50},
		},
		SampledLines: 100,
		ParsedLines:  90,
	}
	opts := &DetectOptions{ShowAll: true}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputDetectText(result, "/test/file.log", opts)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(output, "Alternative formats") {
		t.Error("Expected 'Alternative formats' section")
	}
	if !strings.Contains(output, "Format 2") {
		t.Error("Expected Format 2 in alternatives")
	}
}

func TestOutputDetectJSON(t *testing.T) {
	format := &detector.TimestampFormat{
		Name:       "Test Format",
		PatternStr: "^test",
		Layout:     "2006",
	}
	result := &detector.DetectionResult{
		Matches: []detector.FormatMatch{
			{Format: format, Confidence: 0.95, MatchCount: 95, SampleLine: "test"},
		},
		SampledLines: 100,
		ParsedLines:  95,
	}
	opts := &DetectOptions{}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputDetectJSON(result, "/test/file.log", opts)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(output, `"name": "Test Format"`) {
		t.Error("Expected format name in JSON output")
	}
	if !strings.Contains(output, `"file": "/test/file.log"`) {
		t.Error("Expected file path in JSON output")
	}
}

func TestRunDetect_MissingFile(t *testing.T) {
	cmd := NewDetectCommand()
	cmd.SetArgs([]string{"/nonexistent/file.log"})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestRunDetect_Success(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create log file with timestamps
	content := `2024-01-15T10:30:00 Event 1
2024-01-15T10:30:01 Event 2
2024-01-15T10:30:02 Event 3
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	cmd := NewDetectCommand()
	cmd.SetArgs([]string{logPath})

	// Suppress output
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("Detect failed: %v", err)
	}
}

func TestRunDetect_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	content := `2024-01-15T10:30:00 Event 1
2024-01-15T10:30:01 Event 2
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	cmd := NewDetectCommand()
	cmd.SetArgs([]string{"-o", "json", logPath})
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("Detect with JSON output failed: %v", err)
	}
}

func TestRunDetect_WriteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	configPath := filepath.Join(tmpDir, "output.yaml")

	content := `2024-01-15T10:30:00 Event 1
2024-01-15T10:30:01 Event 2
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	cmd := NewDetectCommand()
	cmd.SetArgs([]string{"--write-config", configPath, logPath})
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("Detect with write-config failed: %v", err)
	}

	// Verify config was written
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}
