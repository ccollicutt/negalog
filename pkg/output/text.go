package output

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ccollicutt/negalog/pkg/analyzer"
)

// TextFormatter formats reports as human-readable text.
type TextFormatter struct {
	opts FormatOptions
}

// NewTextFormatter creates a new text formatter with the given options.
func NewTextFormatter(opts FormatOptions) *TextFormatter {
	return &TextFormatter{opts: opts}
}

// Name returns the format name.
func (f *TextFormatter) Name() string {
	return "text"
}

// Format renders the report as text.
func (f *TextFormatter) Format(ctx context.Context, report *Report, w io.Writer) error {
	if f.opts.Quiet {
		return f.formatQuiet(report, w)
	}
	return f.formatFull(report, w)
}

func (f *TextFormatter) formatQuiet(report *Report, w io.Writer) error {
	fmt.Fprintf(w, "NegaLog: %d rules checked, %d with issues, %d total issues\n",
		report.Summary.RulesChecked,
		report.Summary.RulesWithIssues,
		report.Summary.TotalIssues)
	return nil
}

func (f *TextFormatter) formatFull(report *Report, w io.Writer) error {
	// Header
	fmt.Fprintln(w, "=== NegaLog Analysis Report ===")
	fmt.Fprintln(w)

	// Results by rule
	for _, result := range report.Results {
		if err := f.formatRuleResult(result, w); err != nil {
			return err
		}
	}

	// Summary
	fmt.Fprintln(w, "---")
	fmt.Fprintf(w, "Summary: %d rules checked, %d rules with issues, %d total issues\n",
		report.Summary.RulesChecked,
		report.Summary.RulesWithIssues,
		report.Summary.TotalIssues)

	if f.opts.Verbose {
		fmt.Fprintf(w, "Lines processed: %d\n", report.Summary.LinesProcessed)
		fmt.Fprintf(w, "Duration: %s\n", report.Metadata.Duration.Round(1e6))
	}

	return nil
}

func (f *TextFormatter) formatRuleResult(result *analyzer.RuleResult, w io.Writer) error {
	// Rule header
	ruleType := strings.ToUpper(string(result.RuleType))
	fmt.Fprintf(w, "[%s] %s\n", ruleType, result.RuleName)

	if result.Description != "" && f.opts.Verbose {
		fmt.Fprintf(w, "  %s\n", result.Description)
	}

	if !result.HasIssues() {
		fmt.Fprintln(w, "  No issues detected")
		fmt.Fprintln(w)
		return nil
	}

	// Issue count summary
	fmt.Fprintf(w, "  Missing: %d issue(s)\n", len(result.Issues))

	// Individual issues
	for _, issue := range result.Issues {
		f.formatIssue(&issue, w)
	}

	fmt.Fprintln(w)
	return nil
}

func (f *TextFormatter) formatIssue(issue *analyzer.Issue, w io.Writer) {
	switch issue.Type {
	case analyzer.IssueTypeMissingEnd:
		f.formatMissingEnd(issue, w)
	case analyzer.IssueTypeGapExceeded:
		f.formatGapExceeded(issue, w)
	case analyzer.IssueTypeMissingConsequence:
		f.formatMissingConsequence(issue, w)
	case analyzer.IssueTypeBelowMinOccurrences:
		f.formatBelowMinOccurrences(issue, w)
	default:
		fmt.Fprintf(w, "  - %s\n", issue.Description)
	}
}

func (f *TextFormatter) formatMissingEnd(issue *analyzer.Issue, w io.Writer) {
	ctx := issue.Context
	if ctx.CorrelationID != "" {
		fmt.Fprintf(w, "  - id=%s: started at %s, no end (timeout: %s)\n",
			ctx.CorrelationID,
			ctx.StartTime.Format("15:04:05"),
			ctx.Timeout)
	} else {
		fmt.Fprintf(w, "  - started at %s, no end (timeout: %s)\n",
			ctx.StartTime.Format("15:04:05"),
			ctx.Timeout)
	}

	if f.opts.Verbose {
		fmt.Fprintf(w, "    Source: %s:%d\n", ctx.Source, ctx.LineNum)
	}
}

func (f *TextFormatter) formatGapExceeded(issue *analyzer.Issue, w io.Writer) {
	ctx := issue.Context
	fmt.Fprintf(w, "  - Gap of %s between %s and %s (max allowed: %s)\n",
		ctx.ActualGap.Round(1e9),
		ctx.StartTime.Format("15:04:05"),
		ctx.EndTime.Format("15:04:05"),
		ctx.ExpectedGap)

	if f.opts.Verbose {
		fmt.Fprintf(w, "    Source: %s:%d\n", ctx.Source, ctx.LineNum)
	}
}

func (f *TextFormatter) formatMissingConsequence(issue *analyzer.Issue, w io.Writer) {
	ctx := issue.Context
	if ctx.CorrelationID != "" {
		fmt.Fprintf(w, "  - trigger id=%s at %s: no consequence (timeout: %s)\n",
			ctx.CorrelationID,
			ctx.StartTime.Format("15:04:05"),
			ctx.Timeout)
	} else {
		fmt.Fprintf(w, "  - trigger at %s: no consequence (timeout: %s)\n",
			ctx.StartTime.Format("15:04:05"),
			ctx.Timeout)
	}

	if f.opts.Verbose {
		fmt.Fprintf(w, "    Source: %s:%d\n", ctx.Source, ctx.LineNum)
	}
}

func (f *TextFormatter) formatBelowMinOccurrences(issue *analyzer.Issue, w io.Writer) {
	ctx := issue.Context
	fmt.Fprintf(w, "  - Only %d occurrences (minimum required: %d)\n",
		ctx.Occurrences, ctx.MinRequired)
}
