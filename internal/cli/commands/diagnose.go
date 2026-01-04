package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"negalog/pkg/config"
	"negalog/pkg/detector"
	"negalog/pkg/parser"

	"github.com/spf13/cobra"
)

// DiagnoseOptions holds options for the diagnose command
type DiagnoseOptions struct {
	Verbose bool
}

// DiagnosticResult represents the result of a single diagnostic check
type DiagnosticResult struct {
	Check    string
	Status   string // "ok", "warning", "error"
	Message  string
	Details  []string
	Suggests []string
}

// NewDiagnoseCommand creates the diagnose command
func NewDiagnoseCommand() *cobra.Command {
	opts := &DiagnoseOptions{}

	cmd := &cobra.Command{
		Use:   "diagnose <config-file>",
		Short: "Diagnose common configuration issues",
		Long: `Diagnose common configuration issues.

This command checks your configuration file for common problems:
- Config file syntax and structure
- Log source file existence and accessibility
- Timestamp format matching against actual logs
- Rule configuration validity

Example:
  negalog diagnose config.yaml
  negalog diagnose -v config.yaml  # verbose output`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiagnose(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show detailed diagnostic output")

	return cmd
}

func runDiagnose(ctx context.Context, configPath string, opts *DiagnoseOptions) error {
	results := []DiagnosticResult{}

	// 1. Check config file existence
	result := checkConfigExists(configPath)
	results = append(results, result)
	if result.Status == "error" {
		printDiagnostics(results, opts)
		return nil
	}

	// 2. Parse config file
	cfg, result := checkConfigParseable(configPath)
	results = append(results, result)
	if result.Status == "error" {
		printDiagnostics(results, opts)
		return nil
	}

	// 3. Check log sources
	logResults := checkLogSources(cfg)
	results = append(results, logResults...)

	// 4. Check timestamp format against actual logs
	tsResults := checkTimestampFormat(cfg, opts)
	results = append(results, tsResults...)

	// 5. Check rules configuration
	ruleResults := checkRules(cfg)
	results = append(results, ruleResults...)

	// 6. Check webhooks configuration
	webhookResults := checkWebhooks(cfg, opts)
	results = append(results, webhookResults...)

	printDiagnostics(results, opts)
	return nil
}

func checkConfigExists(path string) DiagnosticResult {
	result := DiagnosticResult{
		Check: "Config File",
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		result.Status = "error"
		result.Message = fmt.Sprintf("Config file not found: %s", path)
		result.Suggests = []string{
			"Check the file path is correct",
			"Use 'negalog detect <log-file> --write-config config.yaml' to generate a starter config",
		}
		return result
	}
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Cannot access config file: %v", err)
		result.Suggests = []string{"Check file permissions"}
		return result
	}
	if info.IsDir() {
		result.Status = "error"
		result.Message = "Path is a directory, not a file"
		return result
	}
	if info.Size() == 0 {
		result.Status = "error"
		result.Message = "Config file is empty"
		result.Suggests = []string{
			"Use 'negalog detect <log-file> --write-config config.yaml' to generate a starter config",
		}
		return result
	}

	result.Status = "ok"
	result.Message = fmt.Sprintf("Found: %s (%d bytes)", path, info.Size())
	return result
}

func checkConfigParseable(path string) (*config.Config, DiagnosticResult) {
	result := DiagnosticResult{
		Check: "Config Syntax",
	}

	cfg, err := config.Load(context.Background(), path)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Failed to parse config: %v", err)
		if strings.Contains(err.Error(), "yaml") {
			result.Suggests = []string{
				"Check YAML syntax - ensure proper indentation (use spaces, not tabs)",
				"Validate YAML at https://yamlvalidator.com/",
			}
		}
		return nil, result
	}

	result.Status = "ok"
	result.Message = "Config file parsed successfully"
	result.Details = []string{
		fmt.Sprintf("Log sources: %d", len(cfg.LogSources)),
		fmt.Sprintf("Rules: %d", len(cfg.Rules)),
	}
	return cfg, result
}

func checkLogSources(cfg *config.Config) []DiagnosticResult {
	results := []DiagnosticResult{}

	if len(cfg.LogSources) == 0 {
		results = append(results, DiagnosticResult{
			Check:   "Log Sources",
			Status:  "error",
			Message: "No log sources defined",
			Suggests: []string{
				"Add log_sources section to your config",
				"Example: log_sources:\n  - /var/log/app/*.log",
			},
		})
		return results
	}

	totalFiles := 0
	for _, source := range cfg.LogSources {
		result := DiagnosticResult{
			Check: fmt.Sprintf("Log Source: %s", source),
		}

		// Check if it's a glob pattern
		if strings.Contains(source, "*") || strings.Contains(source, "?") {
			matches, err := filepath.Glob(source)
			if err != nil {
				result.Status = "error"
				result.Message = fmt.Sprintf("Invalid glob pattern: %v", err)
			} else if len(matches) == 0 {
				result.Status = "warning"
				result.Message = "Glob pattern matches no files"
				result.Suggests = []string{
					"Check if the log files exist at this path",
					"Verify the glob pattern syntax",
				}
			} else {
				result.Status = "ok"
				result.Message = fmt.Sprintf("Matches %d file(s)", len(matches))
				result.Details = append(result.Details, matches...)
				totalFiles += len(matches)
			}
		} else {
			// Direct file path
			info, err := os.Stat(source)
			if os.IsNotExist(err) {
				result.Status = "error"
				result.Message = "File does not exist"
				result.Suggests = []string{
					"Check if the log file path is correct",
					"Use 'ls -la' to verify the file exists",
				}
			} else if err != nil {
				result.Status = "error"
				result.Message = fmt.Sprintf("Cannot access file: %v", err)
				result.Suggests = []string{"Check file permissions"}
			} else if info.IsDir() {
				result.Status = "error"
				result.Message = "Path is a directory, not a file"
				result.Suggests = []string{
					"Use a glob pattern to match files in directory",
					"Example: /var/log/app/*.log",
				}
			} else if info.Size() == 0 {
				result.Status = "warning"
				result.Message = "File is empty (0 bytes)"
			} else {
				result.Status = "ok"
				result.Message = fmt.Sprintf("File exists (%d bytes)", info.Size())
				totalFiles++
			}
		}
		results = append(results, result)
	}

	if totalFiles == 0 {
		results = append(results, DiagnosticResult{
			Check:   "Log Files Summary",
			Status:  "error",
			Message: "No accessible log files found",
			Suggests: []string{
				"Ensure at least one log file exists and is readable",
			},
		})
	}

	return results
}

func checkTimestampFormat(cfg *config.Config, opts *DiagnoseOptions) []DiagnosticResult {
	results := []DiagnosticResult{}

	result := DiagnosticResult{
		Check: "Timestamp Format",
	}

	if cfg.TimestampFormat.Pattern == "" {
		result.Status = "error"
		result.Message = "No timestamp pattern defined"
		result.Suggests = []string{
			"Add timestamp_format section with pattern and layout",
			"Use 'negalog detect <log-file>' to auto-detect the format",
		}
		results = append(results, result)
		return results
	}

	// Try to compile the pattern
	compiledPattern := cfg.TimestampFormat.CompiledPattern()
	if compiledPattern == nil {
		result.Status = "error"
		result.Message = "Invalid timestamp pattern: failed to compile regex"
		result.Suggests = []string{
			"Ensure the regex pattern is valid",
			"Use 'negalog detect <log-file>' to get a working pattern",
		}
		results = append(results, result)
		return results
	}
	tsExtractor := parser.NewTimestampExtractor(compiledPattern, cfg.TimestampFormat.Layout)

	result.Status = "ok"
	result.Message = "Timestamp pattern is valid"
	result.Details = []string{
		fmt.Sprintf("Pattern: %s", cfg.TimestampFormat.Pattern),
		fmt.Sprintf("Layout: %s", cfg.TimestampFormat.Layout),
	}
	results = append(results, result)

	// Test against actual log files
	for _, source := range cfg.LogSources {
		files, _ := filepath.Glob(source)
		if len(files) == 0 {
			continue
		}

		// Test first file
		logFile := files[0]
		testResult := DiagnosticResult{
			Check: fmt.Sprintf("Pattern Test: %s", filepath.Base(logFile)),
		}

		// Read first few lines
		content, err := os.ReadFile(logFile) // #nosec G304 -- user-provided log paths from config
		if err != nil {
			testResult.Status = "warning"
			testResult.Message = fmt.Sprintf("Cannot read file: %v", err)
			results = append(results, testResult)
			continue
		}

		lines := strings.Split(string(content), "\n")
		if len(lines) > 10 {
			lines = lines[:10]
		}

		matchCount := 0
		var sampleMatch string
		var sampleFail string
		for _, line := range lines {
			if line == "" {
				continue
			}
			ts, err := tsExtractor.Extract(line)
			if err == nil && !ts.IsZero() {
				matchCount++
				if sampleMatch == "" {
					sampleMatch = line
				}
			} else if sampleFail == "" {
				sampleFail = line
			}
		}

		if matchCount == 0 {
			testResult.Status = "error"
			testResult.Message = "Pattern matches no lines in log file"
			testResult.Suggests = []string{
				"The timestamp pattern may not match your log format",
				"Use 'negalog detect " + logFile + "' to find the correct pattern",
			}
			if sampleFail != "" {
				testResult.Details = []string{
					"Sample line that didn't match:",
					truncate(sampleFail, 80),
				}
			}

			// Auto-detect and suggest
			d := detector.New(detector.WithSampleSize(10))
			detResult, _ := d.DetectFromFile(context.Background(), logFile)
			if detResult != nil && len(detResult.Matches) > 0 {
				best := detResult.Matches[0]
				testResult.Suggests = append(testResult.Suggests,
					fmt.Sprintf("Detected format: %s", best.Format.Name),
					fmt.Sprintf("Suggested pattern: %s", best.Format.PatternStr),
					fmt.Sprintf("Suggested layout: %s", best.Format.Layout),
				)
			}
		} else if matchCount < len(lines)/2 {
			testResult.Status = "warning"
			testResult.Message = fmt.Sprintf("Pattern matches only %d/%d sample lines", matchCount, len(lines))
			if sampleFail != "" {
				testResult.Details = []string{
					"Sample line that didn't match:",
					truncate(sampleFail, 80),
				}
			}
		} else {
			testResult.Status = "ok"
			testResult.Message = fmt.Sprintf("Pattern matches %d/%d sample lines", matchCount, len(lines))
			if opts.Verbose && sampleMatch != "" {
				testResult.Details = []string{
					"Sample match:",
					truncate(sampleMatch, 80),
				}
			}
		}

		results = append(results, testResult)
		break // Only test first matching file
	}

	return results
}

func checkRules(cfg *config.Config) []DiagnosticResult {
	results := []DiagnosticResult{}

	if len(cfg.Rules) == 0 {
		results = append(results, DiagnosticResult{
			Check:   "Rules",
			Status:  "warning",
			Message: "No detection rules defined",
			Suggests: []string{
				"Add rules to detect missing log patterns",
				"See README.md for rule examples",
			},
		})
		return results
	}

	for _, rule := range cfg.Rules {
		result := DiagnosticResult{
			Check: fmt.Sprintf("Rule: %s", rule.Name),
		}

		issues := []string{}
		warnings := []string{}

		// Check rule type
		switch rule.Type {
		case "sequence":
			if rule.StartPattern == "" {
				issues = append(issues, "Missing start_pattern")
			}
			if rule.EndPattern == "" {
				issues = append(issues, "Missing end_pattern")
			}
			if rule.CorrelationField == 0 {
				warnings = append(warnings, "No correlation_field - all sequences will be matched together")
			}
			if rule.Timeout == 0 {
				issues = append(issues, "Missing timeout")
			}
		case "periodic":
			if rule.Pattern == "" {
				issues = append(issues, "Missing pattern")
			}
			if rule.MaxGap == 0 {
				issues = append(issues, "Missing max_gap")
			}
		case "conditional":
			if rule.TriggerPattern == "" {
				issues = append(issues, "Missing trigger_pattern")
			}
			if rule.ExpectedPattern == "" {
				issues = append(issues, "Missing expected_pattern")
			}
			if rule.Timeout == 0 {
				issues = append(issues, "Missing timeout")
			}
		default:
			issues = append(issues, fmt.Sprintf("Unknown rule type: %s (expected: sequence, periodic, conditional)", rule.Type))
		}

		if len(issues) > 0 {
			result.Status = "error"
			result.Message = fmt.Sprintf("%d configuration issue(s)", len(issues))
			result.Details = issues
		} else if len(warnings) > 0 {
			result.Status = "warning"
			result.Message = fmt.Sprintf("%d warning(s)", len(warnings))
			result.Details = warnings
		} else {
			result.Status = "ok"
			result.Message = fmt.Sprintf("Type: %s", rule.Type)
		}

		results = append(results, result)
	}

	return results
}

func printDiagnostics(results []DiagnosticResult, opts *DiagnoseOptions) {
	fmt.Println("=== NegaLog Configuration Diagnostics ===")
	fmt.Println()

	okCount := 0
	warnCount := 0
	errCount := 0

	for _, r := range results {
		// Status icon
		var icon string
		switch r.Status {
		case "ok":
			icon = "PASS"
			okCount++
		case "warning":
			icon = "WARN"
			warnCount++
		case "error":
			icon = "FAIL"
			errCount++
		}

		fmt.Printf("[%s] %s\n", icon, r.Check)
		fmt.Printf("    %s\n", r.Message)

		if opts.Verbose || r.Status != "ok" {
			for _, d := range r.Details {
				fmt.Printf("      - %s\n", d)
			}
		}

		for _, s := range r.Suggests {
			fmt.Printf("      Hint: %s\n", s)
		}

		fmt.Println()
	}

	// Summary
	fmt.Println("---")
	fmt.Printf("Summary: %d passed, %d warnings, %d errors\n", okCount, warnCount, errCount)

	if errCount > 0 {
		fmt.Println("\nFix the errors above before running analysis.")
	} else if warnCount > 0 {
		fmt.Println("\nConfiguration is usable but has warnings.")
	} else {
		fmt.Println("\nConfiguration looks good!")
	}
}

func checkWebhooks(cfg *config.Config, opts *DiagnoseOptions) []DiagnosticResult {
	results := []DiagnosticResult{}

	if len(cfg.Webhooks) == 0 {
		// Webhooks are optional, just note they're not configured
		if opts.Verbose {
			results = append(results, DiagnosticResult{
				Check:   "Webhooks",
				Status:  "ok",
				Message: "No webhooks configured (optional)",
			})
		}
		return results
	}

	for _, wh := range cfg.Webhooks {
		name := wh.Name
		if name == "" {
			name = wh.URL
		}

		result := DiagnosticResult{
			Check: fmt.Sprintf("Webhook: %s", name),
		}

		issues := []string{}
		warnings := []string{}

		// Check URL
		if wh.URL == "" {
			issues = append(issues, "Missing url")
		} else {
			u, err := url.Parse(wh.URL)
			if err != nil {
				issues = append(issues, fmt.Sprintf("Invalid URL: %v", err))
			} else if u.Scheme != "http" && u.Scheme != "https" {
				issues = append(issues, fmt.Sprintf("URL scheme must be http or https, got %q", u.Scheme))
			} else if u.Host == "" {
				issues = append(issues, "URL must have a host")
			}
		}

		// Check trigger
		if wh.Trigger != "" {
			switch wh.Trigger {
			case config.WebhookTriggerOnIssues, config.WebhookTriggerAlways, config.WebhookTriggerNever:
				// Valid
			default:
				issues = append(issues, fmt.Sprintf("Invalid trigger %q (use on_issues, always, or never)", wh.Trigger))
			}
		}

		// Check if token looks like an unexpanded env var
		if strings.HasPrefix(wh.Token, "${") || strings.HasPrefix(wh.Token, "$") {
			warnings = append(warnings, fmt.Sprintf("Token appears to be an unresolved env var: %s", wh.Token))
		}

		if len(issues) > 0 {
			result.Status = "error"
			result.Message = fmt.Sprintf("%d configuration issue(s)", len(issues))
			result.Details = issues
		} else if len(warnings) > 0 {
			result.Status = "warning"
			result.Message = fmt.Sprintf("%d warning(s)", len(warnings))
			result.Details = warnings
		} else {
			result.Status = "ok"
			result.Message = fmt.Sprintf("Trigger: %s", wh.Trigger)
			if opts.Verbose {
				result.Details = []string{
					fmt.Sprintf("URL: %s", wh.URL),
					fmt.Sprintf("Timeout: %s", wh.Timeout),
				}
				if wh.Token != "" {
					result.Details = append(result.Details, "Token: configured")
				}
			}
		}

		results = append(results, result)
	}

	// Optionally test webhook connectivity
	if opts.Verbose {
		for _, wh := range cfg.Webhooks {
			if wh.URL == "" {
				continue
			}

			name := wh.Name
			if name == "" {
				name = wh.URL
			}

			result := checkWebhookConnectivity(wh)
			result.Check = fmt.Sprintf("Webhook Connectivity: %s", name)
			results = append(results, result)
		}
	}

	return results
}

func checkWebhookConnectivity(wh config.WebhookConfig) DiagnosticResult {
	result := DiagnosticResult{}

	// Just do a HEAD request to check if the endpoint is reachable
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodHead, wh.URL, nil)
	if err != nil {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Cannot create request: %v", err)
		return result
	}

	if wh.Token != "" {
		req.Header.Set("Authorization", "Bearer "+wh.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Cannot connect: %v", err)
		result.Suggests = []string{
			"Check if the webhook URL is correct",
			"Verify network connectivity",
		}
		return result
	}
	defer resp.Body.Close()

	// Any response (even 4xx/5xx) means the server is reachable
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		result.Status = "ok"
		result.Message = fmt.Sprintf("Reachable (status %d)", resp.StatusCode)
	} else {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Reachable but returned status %d", resp.StatusCode)
		result.Suggests = []string{
			"The endpoint may require POST method (will work during actual webhook send)",
			"Check authentication if using a token",
		}
	}

	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
