package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"negalog/pkg/analyzer"
	"negalog/pkg/config"
	"negalog/pkg/output"
	"negalog/pkg/parser"
	"negalog/pkg/webhook"
)

// ExitCode is set by commands to indicate the result
var ExitCode = 0

// AnalyzeOptions holds command-line options for the analyze command.
type AnalyzeOptions struct {
	Output    string
	TimeRange string
	Rules     []string
	Verbose   bool
	Quiet     bool

	// Webhook options
	WebhookURL     string
	WebhookToken   string
	WebhookTrigger string
}

// NewAnalyzeCommand creates the analyze command.
func NewAnalyzeCommand() *cobra.Command {
	opts := &AnalyzeOptions{}

	cmd := &cobra.Command{
		Use:   "analyze <config-file>",
		Short: "Analyze logs for missing entries",
		Long: `Analyze log files according to rules defined in the configuration file.

Detects:
  - Sequence gaps (start without matching end)
  - Periodic absence (missing recurring logs)
  - Conditional absence (trigger without expected consequence)

Exit codes:
  0 - No missing logs detected
  1 - Missing logs detected
  2 - Configuration or runtime error`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(cmd, args, opts)
		},
	}

	// Flags
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().StringVar(&opts.TimeRange, "time-range", "", "Limit analysis to time window (e.g., 2h, 24h)")
	cmd.Flags().StringSliceVar(&opts.Rules, "rule", nil, "Run specific rule(s) only (can be repeated)")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show matched logs, not just missing ones")
	cmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false, "Summary only, no details")

	// Webhook flags
	cmd.Flags().StringVar(&opts.WebhookURL, "webhook-url", "", "Webhook endpoint URL")
	cmd.Flags().StringVar(&opts.WebhookToken, "webhook-token", "", "Bearer token for webhook auth")
	cmd.Flags().StringVar(&opts.WebhookTrigger, "webhook-trigger", "on_issues", "When to fire webhook (on_issues|always|never)")

	return cmd
}

func runAnalyze(cmd *cobra.Command, args []string, opts *AnalyzeOptions) error {
	configPath := args[0]
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Load configuration
	cfg, err := config.Load(ctx, configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Expand log source globs
	files, err := parser.ExpandGlobs(cfg.LogSources)
	if err != nil {
		return fmt.Errorf("expanding log sources: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no log files matched patterns: %v", cfg.LogSources)
	}

	// Parse time range if specified
	var analyzerOpts []analyzer.AnalyzerOption

	if opts.TimeRange != "" {
		duration, err := time.ParseDuration(opts.TimeRange)
		if err != nil {
			return fmt.Errorf("invalid time-range %q: %w", opts.TimeRange, err)
		}
		end := time.Now()
		start := end.Add(-duration)
		analyzerOpts = append(analyzerOpts, analyzer.WithTimeRange(start, end))
	}

	if len(opts.Rules) > 0 {
		analyzerOpts = append(analyzerOpts, analyzer.WithRuleFilter(opts.Rules))
	}

	analyzerOpts = append(analyzerOpts, analyzer.WithVerbose(opts.Verbose))

	// Create analyzer
	a, err := analyzer.NewAnalyzer(cfg, analyzerOpts...)
	if err != nil {
		return fmt.Errorf("creating analyzer: %w", err)
	}

	// Create log source with timestamp-ordered merging across files
	var source parser.LogSource
	if len(files) == 1 {
		// Single file - use simple FileSource
		source = parser.NewFileSource(
			files,
			cfg.TimestampFormat.CompiledPattern(),
			cfg.TimestampFormat.Layout,
		)
	} else {
		// Multiple files - use MergedSource for chronological ordering
		sources := make([]parser.LogSource, len(files))
		for i, file := range files {
			sources[i] = parser.NewFileSource(
				[]string{file},
				cfg.TimestampFormat.CompiledPattern(),
				cfg.TimestampFormat.Layout,
			)
		}
		source = parser.NewMergedSource(sources...)
	}
	defer source.Close()

	// Run analysis
	result, err := a.Analyze(ctx, source)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Create report
	report := output.NewReport(result, configPath)

	// Create formatter
	formatter, err := createFormatter(opts)
	if err != nil {
		return err
	}

	// Output report
	if err := formatter.Format(ctx, report, os.Stdout); err != nil {
		return fmt.Errorf("formatting output: %w", err)
	}

	// Send webhooks (errors logged but don't fail analysis)
	sendWebhooks(ctx, cfg, opts, report)

	// Set exit code based on results
	if report.HasIssues() {
		ExitCode = 1
	}

	return nil
}

func createFormatter(opts *AnalyzeOptions) (output.Formatter, error) {
	formatOpts := output.FormatOptions{
		Verbose: opts.Verbose,
		Quiet:   opts.Quiet,
	}

	switch opts.Output {
	case "text":
		return output.NewTextFormatter(formatOpts), nil
	case "json":
		return output.NewJSONFormatter(formatOpts), nil
	default:
		return nil, fmt.Errorf("unknown output format %q (use text or json)", opts.Output)
	}
}

// sendWebhooks sends the report to all configured webhooks.
// Errors are logged to stderr but don't fail the analysis.
func sendWebhooks(ctx context.Context, cfg *config.Config, opts *AnalyzeOptions, report *output.Report) {
	// Collect webhooks from config and CLI
	webhooks := collectWebhooks(cfg, opts)

	if len(webhooks) == 0 {
		return
	}

	client := webhook.NewClient()

	for _, wh := range webhooks {
		// Check trigger condition
		if !shouldFireWebhook(wh.Trigger, report.HasIssues()) {
			continue
		}

		// Send webhook
		resp := client.Send(ctx, report, webhook.SendOptions{
			URL:     wh.URL,
			Token:   wh.Token,
			Timeout: wh.Timeout,
		})

		// Log result
		name := wh.Name
		if name == "" {
			name = wh.URL
		}

		if resp.Success() {
			fmt.Fprintf(os.Stderr, "Webhook %s: sent (%d, %s)\n", name, resp.StatusCode, resp.Duration)
		} else {
			fmt.Fprintf(os.Stderr, "Webhook %s: failed (%v)\n", name, resp.Error)
		}
	}
}

// collectWebhooks merges config file webhooks with CLI webhook.
func collectWebhooks(cfg *config.Config, opts *AnalyzeOptions) []config.WebhookConfig {
	webhooks := make([]config.WebhookConfig, 0, len(cfg.Webhooks)+1)

	// Add config file webhooks
	webhooks = append(webhooks, cfg.Webhooks...)

	// Add CLI webhook if specified
	if opts.WebhookURL != "" {
		trigger := config.WebhookTrigger(opts.WebhookTrigger)
		if trigger == "" {
			trigger = config.WebhookTriggerOnIssues
		}

		webhooks = append(webhooks, config.WebhookConfig{
			Name:    "cli",
			URL:     opts.WebhookURL,
			Token:   opts.WebhookToken,
			Trigger: trigger,
			Timeout: config.DefaultWebhookTimeout,
		})
	}

	return webhooks
}

// shouldFireWebhook determines if a webhook should fire based on trigger and issues.
func shouldFireWebhook(trigger config.WebhookTrigger, hasIssues bool) bool {
	switch trigger {
	case config.WebhookTriggerAlways:
		return true
	case config.WebhookTriggerNever:
		return false
	case config.WebhookTriggerOnIssues:
		return hasIssues
	default:
		// Default to on_issues
		return hasIssues
	}
}
