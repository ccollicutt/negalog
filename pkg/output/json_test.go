package output

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"negalog/pkg/analyzer"
)

func TestNewJSONFormatter(t *testing.T) {
	f := NewJSONFormatter(FormatOptions{})
	if f == nil {
		t.Fatal("NewJSONFormatter() returned nil")
	}
	if f.Name() != "json" {
		t.Errorf("Name() = %q, want %q", f.Name(), "json")
	}
}

func TestJSONFormatter_Format(t *testing.T) {
	f := NewJSONFormatter(FormatOptions{})
	report := createTestReport()

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed Report
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Check content
	if parsed.Summary.RulesChecked != 1 {
		t.Errorf("RulesChecked = %d, want 1", parsed.Summary.RulesChecked)
	}
	if parsed.Summary.TotalIssues != 1 {
		t.Errorf("TotalIssues = %d, want 1", parsed.Summary.TotalIssues)
	}
}

func TestJSONFormatter_Format_Quiet(t *testing.T) {
	f := NewJSONFormatter(FormatOptions{Quiet: true})
	report := createTestReport()

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Quiet mode should only output summary
	var parsed Summary
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if parsed.RulesChecked != 1 {
		t.Errorf("RulesChecked = %d, want 1", parsed.RulesChecked)
	}
}

func TestJSONFormatter_Format_Empty(t *testing.T) {
	f := NewJSONFormatter(FormatOptions{})
	report := &Report{
		Summary: Summary{},
		Results: []*analyzer.RuleResult{},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var parsed Report
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}
}

func TestJSONFormatter_Format_ComplexReport(t *testing.T) {
	f := NewJSONFormatter(FormatOptions{})

	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	report := &Report{
		Summary: Summary{
			RulesChecked:    3,
			RulesWithIssues: 2,
			TotalIssues:     5,
			LinesProcessed:  1000,
		},
		Results: []*analyzer.RuleResult{
			{
				RuleName: "rule1",
				RuleType: analyzer.RuleTypeSequence,
				Issues: []analyzer.Issue{
					{Type: analyzer.IssueTypeMissingEnd},
					{Type: analyzer.IssueTypeMissingEnd},
				},
			},
			{
				RuleName: "rule2",
				RuleType: analyzer.RuleTypePeriodic,
				Issues:   []analyzer.Issue{},
			},
			{
				RuleName: "rule3",
				RuleType: analyzer.RuleTypeConditional,
				Issues: []analyzer.Issue{
					{Type: analyzer.IssueTypeMissingConsequence},
					{Type: analyzer.IssueTypeMissingConsequence},
					{Type: analyzer.IssueTypeMissingConsequence},
				},
			},
		},
		Metadata: Metadata{
			ConfigFile: "config.yaml",
			Sources:    []string{"a.log", "b.log"},
			AnalyzedAt: baseTime,
			Duration:   5 * time.Second,
		},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), report, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var parsed Report
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if len(parsed.Results) != 3 {
		t.Errorf("len(Results) = %d, want 3", len(parsed.Results))
	}
	if len(parsed.Metadata.Sources) != 2 {
		t.Errorf("len(Sources) = %d, want 2", len(parsed.Metadata.Sources))
	}
}
