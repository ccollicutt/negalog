package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ccollicutt/negalog/pkg/detector"
)

// DetectOptions holds command-line options for the detect command.
type DetectOptions struct {
	Output      string
	SampleSize  int
	ShowAll     bool
	WriteConfig string
}

// NewDetectCommand creates the detect command.
func NewDetectCommand() *cobra.Command {
	opts := &DetectOptions{}

	cmd := &cobra.Command{
		Use:   "detect <log-file>",
		Short: "Detect timestamp format in a log file",
		Long: `Analyze a log file to automatically detect its timestamp format.

Samples lines from the file and tests against common timestamp patterns.
Reports the detected format with confidence score and provides a ready-to-use
YAML configuration snippet.

Optionally generates a starter config file with --write-config.

Supports:
  - ISO 8601 variants (with/without timezone, milliseconds)
  - Syslog format (BSD and with year)
  - Apache/NGINX common log format
  - Unix timestamps (seconds and milliseconds)
  - Python/Java logging formats
  - Bracketed datetime formats

Example:
  negalog detect /var/log/myapp.log
  negalog detect --sample 500 /var/log/large.log
  negalog detect --write-config myapp.yaml /var/log/app.log
  negalog detect -w negalog.yaml /var/log/app.log`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetect(cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Output, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().IntVarP(&opts.SampleSize, "sample", "n", 100, "Number of lines to sample")
	cmd.Flags().BoolVar(&opts.ShowAll, "all", false, "Show all detected formats, not just the best match")
	cmd.Flags().StringVarP(&opts.WriteConfig, "write-config", "w", "", "Write starter config to file (will not overwrite)")

	return cmd
}

func runDetect(cmd *cobra.Command, args []string, opts *DetectOptions) error {
	logFile := args[0]
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Check file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s", logFile)
	}

	// Create detector
	d := detector.New(detector.WithSampleSize(opts.SampleSize))

	// Run detection
	result, err := d.DetectFromFile(ctx, logFile)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	// Write config file if requested
	if opts.WriteConfig != "" {
		if err := writeStarterConfig(result, logFile, opts.WriteConfig); err != nil {
			return err
		}
	}

	// Output results
	switch opts.Output {
	case "json":
		return outputDetectJSON(result, logFile, opts)
	default:
		return outputDetectText(result, logFile, opts)
	}
}

func outputDetectText(result *detector.DetectionResult, logFile string, opts *DetectOptions) error {
	fmt.Println("=== Timestamp Format Detection ===")
	fmt.Println()
	fmt.Printf("File: %s\n", logFile)
	fmt.Printf("Lines sampled: %d\n", result.SampledLines)
	fmt.Printf("Lines with timestamps: %d\n", result.ParsedLines)
	fmt.Println()

	if !result.HasMatch() {
		fmt.Println("No timestamp format detected.")
		fmt.Println()
		fmt.Println("Tip: The file may use an uncommon format.")
		fmt.Println("Check the first few lines manually to identify the timestamp pattern.")
		return nil
	}

	// Show best match
	best := result.BestMatch()
	fmt.Printf("Detected Format: %s\n", best.Format.Name)
	fmt.Printf("Confidence: %.1f%% (%d/%d lines matched)\n",
		best.Confidence*100, best.MatchCount, result.SampledLines)
	fmt.Println()
	fmt.Printf("Sample match:\n  %s\n", best.SampleLine)
	fmt.Printf("Parsed as: %s\n", best.ParsedTime.Format("2006-01-02 15:04:05 MST"))
	fmt.Println()

	// Ambiguity warning
	if best.Format.Ambiguous {
		fmt.Println("WARNING: This format has date ordering ambiguity (MM/DD vs DD/MM).")
		fmt.Println("Please verify the layout matches your log format.")
		fmt.Println()
	}
	if result.AmbiguityNote != "" {
		fmt.Printf("Note: %s\n", result.AmbiguityNote)
		fmt.Println()
	}

	// YAML snippet
	fmt.Println("--- Configuration snippet (copy to your config file) ---")
	fmt.Println()
	fmt.Println("timestamp_format:")
	fmt.Printf("  pattern: '%s'\n", best.Format.PatternStr)
	fmt.Printf("  layout: \"%s\"\n", best.Format.Layout)
	fmt.Println()

	// Show alternatives if requested
	if opts.ShowAll && len(result.Matches) > 1 {
		fmt.Println("--- Alternative formats detected ---")
		for i, m := range result.Matches[1:] {
			fmt.Printf("%d. %s (%.1f%% confidence)\n", i+2, m.Format.Name, m.Confidence*100)
			fmt.Printf("   pattern: '%s'\n", m.Format.PatternStr)
			fmt.Printf("   layout: \"%s\"\n", m.Format.Layout)
		}
		fmt.Println()
	}

	return nil
}

// JSONMatch represents a format match in JSON output.
type JSONMatch struct {
	Name       string  `json:"name"`
	Pattern    string  `json:"pattern"`
	Layout     string  `json:"layout"`
	Confidence float64 `json:"confidence"`
	MatchCount int     `json:"match_count"`
	SampleLine string  `json:"sample_line"`
	Ambiguous  bool    `json:"ambiguous,omitempty"`
}

// JSONOutput represents the full JSON output.
type JSONOutput struct {
	File          string      `json:"file"`
	Matches       []JSONMatch `json:"matches"`
	SampledLines  int         `json:"sampled_lines"`
	ParsedLines   int         `json:"parsed_lines"`
	AmbiguityNote string      `json:"ambiguity_note,omitempty"`
}

func outputDetectJSON(result *detector.DetectionResult, logFile string, opts *DetectOptions) error {
	output := JSONOutput{
		File:          logFile,
		SampledLines:  result.SampledLines,
		ParsedLines:   result.ParsedLines,
		AmbiguityNote: result.AmbiguityNote,
		Matches:       make([]JSONMatch, 0),
	}

	matches := result.Matches
	if !opts.ShowAll && len(matches) > 1 {
		matches = matches[:1] // Only show best match
	}

	for _, m := range matches {
		output.Matches = append(output.Matches, JSONMatch{
			Name:       m.Format.Name,
			Pattern:    m.Format.PatternStr,
			Layout:     m.Format.Layout,
			Confidence: m.Confidence,
			MatchCount: m.MatchCount,
			SampleLine: m.SampleLine,
			Ambiguous:  m.Format.Ambiguous,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// writeStarterConfig generates a starter config file with the detected format.
func writeStarterConfig(result *detector.DetectionResult, logFile, configPath string) error {
	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists: %s (will not overwrite)", configPath)
	}

	// Need a detected format to generate config
	if !result.HasMatch() {
		return fmt.Errorf("cannot generate config: no timestamp format detected")
	}

	best := result.BestMatch()

	// Generate the config content
	config := generateStarterConfig(logFile, best)

	// Write the file
	// #nosec G306 - config file doesn't need restrictive permissions
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Wrote starter config to: %s\n\n", configPath)
	return nil
}

// generateStarterConfig creates a YAML config template.
func generateStarterConfig(logFile string, match *detector.FormatMatch) string {
	// Get absolute path for log file if possible
	absLogFile := logFile
	if abs, err := filepath.Abs(logFile); err == nil {
		absLogFile = abs
	}

	return fmt.Sprintf(`# NegaLog Configuration
# Generated by: negalog detect
# Detected format: %s (%.0f%% confidence)

log_sources:
  - %s
  # Add more log files or use globs:
  # - /var/log/myapp/*.log

timestamp_format:
  pattern: '%s'
  layout: "%s"

rules:
  # Example: Detect sequences that start but never end
  # - name: unclosed-sessions
  #   type: sequence
  #   description: "Detect sessions opened but never closed"
  #   start_pattern: 'session opened for user (\w+)'
  #   end_pattern: 'session closed for user (\w+)'
  #   correlation_field: 1
  #   timeout: 1h

  # Example: Detect missing periodic events (heartbeats)
  # - name: missing-heartbeat
  #   type: periodic
  #   description: "Heartbeat should occur every 5 minutes"
  #   pattern: 'HEARTBEAT'
  #   max_gap: 7m
  #   min_occurrences: 10

  # Example: Detect trigger without expected consequence
  # - name: unhandled-error
  #   type: conditional
  #   description: "Errors should be followed by recovery"
  #   trigger_pattern: 'ERROR:.*failed'
  #   expected_pattern: 'recovered|retry succeeded'
  #   timeout: 5m

  # Add your rules here:
  - name: example-rule
    type: sequence
    description: "Example rule - customize or replace"
    start_pattern: 'START'
    end_pattern: 'END'
    timeout: 1h
`, match.Format.Name, match.Confidence*100,
		absLogFile,
		match.Format.PatternStr,
		match.Format.Layout)
}
