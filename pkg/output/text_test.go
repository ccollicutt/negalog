package output

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ccollicutt/negalog/pkg/analyzer"
)

func TestNewTextFormatter(t *testing.T) {
	f := NewTextFormatter(FormatOptions{})
	if f == nil {
		t.Fatal("NewTextFormatter() returned nil")
	}
	if f.Name() != "text" {
		t.Errorf("Name() = %q, want %q", f.Name(), "text")
	}
}

func TestTextFormatter_Format_Empty(t *testing.T) {
	f := NewTextFormatter(FormatOptions{})
	report := &Report{
		Summary: Summary{RulesChecked: 0},
		Results: []*analyzer.RuleResult{},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NegaLog Analysis Report") {
		t.Error("Output missing header")
	}
	if !strings.Contains(output, "0 rules checked") {
		t.Error("Output missing summary")
	}
}

func TestTextFormatter_Format_WithIssues(t *testing.T) {
	f := NewTextFormatter(FormatOptions{})
	report := createTestReport()

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Check header
	if !strings.Contains(output, "[SEQUENCE]") {
		t.Error("Output missing rule type")
	}

	// Check issue details
	if !strings.Contains(output, "abc123") {
		t.Error("Output missing correlation ID")
	}

	// Check summary
	if !strings.Contains(output, "1 rules with issues") {
		t.Error("Output missing summary")
	}
}

func TestTextFormatter_Format_Quiet(t *testing.T) {
	f := NewTextFormatter(FormatOptions{Quiet: true})
	report := createTestReport()

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Quiet mode should be a single line
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("Quiet output has %d lines, want 1", len(lines))
	}

	if !strings.Contains(output, "NegaLog:") {
		t.Error("Quiet output missing prefix")
	}
}

func TestTextFormatter_Format_Verbose(t *testing.T) {
	f := NewTextFormatter(FormatOptions{Verbose: true})
	report := createTestReport()

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Verbose should include source info
	if !strings.Contains(output, "Source:") {
		t.Error("Verbose output missing source info")
	}

	// Should include duration
	if !strings.Contains(output, "Duration:") {
		t.Error("Verbose output missing duration")
	}
}

func TestTextFormatter_Format_AllIssueTypes(t *testing.T) {
	f := NewTextFormatter(FormatOptions{Verbose: true})

	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	report := &Report{
		Summary: Summary{RulesChecked: 4, RulesWithIssues: 4, TotalIssues: 4},
		Results: []*analyzer.RuleResult{
			{
				RuleName: "seq",
				RuleType: analyzer.RuleTypeSequence,
				Issues: []analyzer.Issue{{
					Type: analyzer.IssueTypeMissingEnd,
					Context: analyzer.IssueContext{
						CorrelationID: "abc",
						StartTime:     baseTime,
						Timeout:       60 * time.Second,
						Source:        "test.log",
						LineNum:       1,
					},
				}},
			},
			{
				RuleName: "periodic",
				RuleType: analyzer.RuleTypePeriodic,
				Issues: []analyzer.Issue{{
					Type: analyzer.IssueTypeGapExceeded,
					Context: analyzer.IssueContext{
						StartTime:   baseTime,
						EndTime:     baseTime.Add(10 * time.Minute),
						ActualGap:   10 * time.Minute,
						ExpectedGap: 5 * time.Minute,
						Source:      "test.log",
						LineNum:     2,
					},
				}},
			},
			{
				RuleName: "cond",
				RuleType: analyzer.RuleTypeConditional,
				Issues: []analyzer.Issue{{
					Type: analyzer.IssueTypeMissingConsequence,
					Context: analyzer.IssueContext{
						CorrelationID: "500",
						StartTime:     baseTime,
						Timeout:       10 * time.Second,
						Source:        "test.log",
						LineNum:       3,
					},
				}},
			},
			{
				RuleName: "min",
				RuleType: analyzer.RuleTypePeriodic,
				Issues: []analyzer.Issue{{
					Type: analyzer.IssueTypeBelowMinOccurrences,
					Context: analyzer.IssueContext{
						Occurrences: 3,
						MinRequired: 10,
					},
				}},
			},
		},
		Metadata: Metadata{Duration: 100 * time.Millisecond},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Check all issue types are formatted
	checks := []string{
		"no end",         // MissingEnd
		"Gap of",         // GapExceeded
		"no consequence", // MissingConsequence
		"occurrences",    // BelowMinOccurrences
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("Output missing %q", check)
		}
	}
}

func TestTextFormatter_Format_NoIssues(t *testing.T) {
	f := NewTextFormatter(FormatOptions{})
	report := &Report{
		Summary: Summary{RulesChecked: 1},
		Results: []*analyzer.RuleResult{{
			RuleName: "test",
			RuleType: analyzer.RuleTypeSequence,
			Issues:   []analyzer.Issue{},
		}},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No issues detected") {
		t.Error("Output missing 'No issues detected' message")
	}
}

func createTestReport() *Report {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	return &Report{
		Summary: Summary{
			RulesChecked:    1,
			RulesWithIssues: 1,
			TotalIssues:     1,
			LinesProcessed:  100,
		},
		Results: []*analyzer.RuleResult{{
			RuleName:    "test-rule",
			RuleType:    analyzer.RuleTypeSequence,
			Description: "Test rule description",
			Issues: []analyzer.Issue{{
				Type:        analyzer.IssueTypeMissingEnd,
				Description: "Sequence not completed",
				Context: analyzer.IssueContext{
					CorrelationID: "abc123",
					StartTime:     baseTime,
					Source:        "test.log",
					LineNum:       42,
					Timeout:       60 * time.Second,
				},
			}},
		}},
		Metadata: Metadata{
			ConfigFile: "test.yaml",
			Sources:    []string{"test.log"},
			AnalyzedAt: baseTime,
			Duration:   100 * time.Millisecond,
		},
	}
}
