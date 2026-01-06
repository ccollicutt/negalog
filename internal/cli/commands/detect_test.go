package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ccollicutt/negalog/pkg/detector"
)

func TestGenerateStarterConfig(t *testing.T) {
	// Create a mock format match
	format := &detector.TimestampFormat{
		Name:       "Syslog (BSD)",
		PatternStr: `^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})`,
		Layout:     "Jan 2 15:04:05",
	}
	match := &detector.FormatMatch{
		Format:     format,
		Confidence: 0.95,
	}

	config := generateStarterConfig("/var/log/test.log", match)

	// Verify config contains expected elements
	checks := []string{
		"log_sources:",
		"/var/log/test.log",
		"timestamp_format:",
		"pattern:",
		"layout:",
		"Jan 2 15:04:05",
		"rules:",
		"Syslog (BSD)",
		"95%",
	}

	for _, check := range checks {
		if !strings.Contains(config, check) {
			t.Errorf("Config missing %q", check)
		}
	}
}

func TestWriteStarterConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	// Create a mock result
	format := &detector.TimestampFormat{
		Name:       "ISO 8601",
		PatternStr: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})`,
		Layout:     "2006-01-02T15:04:05",
	}
	result := &detector.DetectionResult{
		Matches: []detector.FormatMatch{
			{
				Format:     format,
				Confidence: 1.0,
				MatchCount: 100,
			},
		},
		SampledLines: 100,
		ParsedLines:  100,
	}

	err := writeStarterConfig(result, "/var/log/app.log", configPath)
	if err != nil {
		t.Fatalf("writeStarterConfig failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if !strings.Contains(string(content), "ISO 8601") {
		t.Error("Config missing format name")
	}
	if !strings.Contains(string(content), "2006-01-02T15:04:05") {
		t.Error("Config missing layout")
	}
}

func TestWriteStarterConfig_NoOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "existing.yaml")

	// Create existing file
	if err := os.WriteFile(configPath, []byte("existing content"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Create a mock result
	format := &detector.TimestampFormat{
		Name:       "ISO 8601",
		PatternStr: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})`,
		Layout:     "2006-01-02T15:04:05",
	}
	result := &detector.DetectionResult{
		Matches: []detector.FormatMatch{
			{Format: format, Confidence: 1.0},
		},
	}

	err := writeStarterConfig(result, "/var/log/app.log", configPath)
	if err == nil {
		t.Error("Expected error when file exists, got nil")
	}
	if !strings.Contains(err.Error(), "will not overwrite") {
		t.Errorf("Expected 'will not overwrite' error, got: %v", err)
	}

	// Verify original content unchanged
	content, _ := os.ReadFile(configPath)
	if string(content) != "existing content" {
		t.Error("Existing file was modified")
	}
}

func TestWriteStarterConfig_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	// Empty result - no matches
	result := &detector.DetectionResult{
		Matches:      []detector.FormatMatch{},
		SampledLines: 100,
		ParsedLines:  0,
	}

	err := writeStarterConfig(result, "/var/log/app.log", configPath)
	if err == nil {
		t.Error("Expected error when no format detected, got nil")
	}
	if !strings.Contains(err.Error(), "no timestamp format detected") {
		t.Errorf("Expected 'no timestamp format detected' error, got: %v", err)
	}
}

func TestDetectOptions_Defaults(t *testing.T) {
	cmd := NewDetectCommand()

	// Check default values
	output, _ := cmd.Flags().GetString("output")
	if output != "text" {
		t.Errorf("Expected default output 'text', got %q", output)
	}

	sample, _ := cmd.Flags().GetInt("sample")
	if sample != 100 {
		t.Errorf("Expected default sample 100, got %d", sample)
	}

	writeConfig, _ := cmd.Flags().GetString("write-config")
	if writeConfig != "" {
		t.Errorf("Expected default write-config '', got %q", writeConfig)
	}
}
