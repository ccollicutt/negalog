package analyzer

import (
	"context"
	"io"
	"testing"
	"time"

	"negalog/pkg/config"
	"negalog/pkg/parser"
)

// mockSource is a test LogSource that returns predefined lines.
type mockSource struct {
	lines []*parser.ParsedLine
	index int
}

func (m *mockSource) Next(ctx context.Context) (*parser.ParsedLine, error) {
	if m.index >= len(m.lines) {
		return nil, io.EOF
	}
	line := m.lines[m.index]
	m.index++
	return line, nil
}

func (m *mockSource) Close() error {
	return nil
}

func TestNewAnalyzer(t *testing.T) {
	cfg := createTestConfig(t)

	a, err := NewAnalyzer(cfg)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	if a == nil {
		t.Fatal("NewAnalyzer() returned nil")
	}
}

func TestNewAnalyzer_NoRules(t *testing.T) {
	cfg := &config.Config{
		LogSources:      []string{"/tmp"},
		TimestampFormat: config.TimestampConfig{Pattern: `^(\d+)`, Layout: "2006"},
		Rules:           []config.RuleConfig{},
	}

	// This should fail validation before reaching NewAnalyzer
	// but let's test with rule filter that excludes all
	cfg.Rules = []config.RuleConfig{{
		Name:             "test",
		Type:             "sequence",
		StartPattern:     `START`,
		EndPattern:       `END`,
		CorrelationField: 0, // Invalid, but we're testing rule filter
	}}

	// Skip validation for this test
	_, err := NewAnalyzer(cfg, WithRuleFilter([]string{"nonexistent"}))
	if err == nil {
		t.Error("NewAnalyzer() expected error when all rules filtered")
	}
}

func TestAnalyzer_Analyze(t *testing.T) {
	cfg := createTestConfig(t)

	a, err := NewAnalyzer(cfg)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	source := &mockSource{
		lines: []*parser.ParsedLine{
			{Raw: "START id=abc", Timestamp: baseTime, Source: "test.log", LineNum: 1},
			{Raw: "END id=abc", Timestamp: baseTime.Add(10 * time.Second), Source: "test.log", LineNum: 2},
			{Raw: "START id=def", Timestamp: baseTime.Add(20 * time.Second), Source: "test.log", LineNum: 3},
			// No END for def
		},
	}

	result, err := a.Analyze(context.Background(), source)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result.TotalIssues() != 1 {
		t.Errorf("TotalIssues() = %d, want 1", result.TotalIssues())
	}

	if result.RulesWithIssues() != 1 {
		t.Errorf("RulesWithIssues() = %d, want 1", result.RulesWithIssues())
	}
}

func TestAnalyzer_WithTimeRange(t *testing.T) {
	cfg := createTestConfig(t)

	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	a, err := NewAnalyzer(cfg, WithTimeRange(
		baseTime.Add(5*time.Second),
		baseTime.Add(25*time.Second),
	))
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	source := &mockSource{
		lines: []*parser.ParsedLine{
			// Before range - should be filtered
			{Raw: "START id=abc", Timestamp: baseTime, Source: "test.log", LineNum: 1},
			// In range
			{Raw: "START id=def", Timestamp: baseTime.Add(10 * time.Second), Source: "test.log", LineNum: 2},
			{Raw: "END id=def", Timestamp: baseTime.Add(20 * time.Second), Source: "test.log", LineNum: 3},
			// After range - should be filtered
			{Raw: "START id=ghi", Timestamp: baseTime.Add(30 * time.Second), Source: "test.log", LineNum: 4},
		},
	}

	result, err := a.Analyze(context.Background(), source)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	// Only def should be processed and it's complete
	if result.TotalIssues() != 0 {
		t.Errorf("TotalIssues() = %d, want 0 (filtered by time range)", result.TotalIssues())
	}

	if result.Metadata.LinesProcessed != 2 {
		t.Errorf("LinesProcessed = %d, want 2", result.Metadata.LinesProcessed)
	}
}

func TestAnalyzer_WithRuleFilter(t *testing.T) {
	cfg := &config.Config{
		LogSources:      []string{"/tmp"},
		TimestampFormat: config.TimestampConfig{Pattern: `^(\d+)`, Layout: "2006"},
		Rules: []config.RuleConfig{
			{
				Name:             "rule1",
				Type:             "sequence",
				StartPattern:     `START1 id=(\w+)`,
				EndPattern:       `END1 id=(\w+)`,
				CorrelationField: 1,
			},
			{
				Name:             "rule2",
				Type:             "sequence",
				StartPattern:     `START2 id=(\w+)`,
				EndPattern:       `END2 id=(\w+)`,
				CorrelationField: 1,
			},
		},
	}

	if err := config.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	// Only run rule1
	a, err := NewAnalyzer(cfg, WithRuleFilter([]string{"rule1"}))
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	source := &mockSource{
		lines: []*parser.ParsedLine{
			{Raw: "START1 id=abc", Timestamp: time.Now(), Source: "test.log"},
			{Raw: "START2 id=def", Timestamp: time.Now(), Source: "test.log"},
		},
	}

	result, err := a.Analyze(context.Background(), source)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	// Should only have 1 rule result (rule1)
	if len(result.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(result.Results))
	}

	if result.Results[0].RuleName != "rule1" {
		t.Errorf("RuleName = %q, want %q", result.Results[0].RuleName, "rule1")
	}
}

func TestAnalyzer_ContextCancellation(t *testing.T) {
	cfg := createTestConfig(t)

	a, err := NewAnalyzer(cfg)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	source := &mockSource{
		lines: []*parser.ParsedLine{
			{Raw: "START id=abc", Timestamp: time.Now()},
		},
	}

	_, err = a.Analyze(ctx, source)
	if err != context.Canceled {
		t.Errorf("Analyze() error = %v, want context.Canceled", err)
	}
}

func TestAnalysisResult_Methods(t *testing.T) {
	result := &AnalysisResult{
		Results: []*RuleResult{
			{RuleName: "r1", Issues: []Issue{{}, {}}},
			{RuleName: "r2", Issues: []Issue{}},
			{RuleName: "r3", Issues: []Issue{{}}},
		},
	}

	if result.TotalIssues() != 3 {
		t.Errorf("TotalIssues() = %d, want 3", result.TotalIssues())
	}

	if result.RulesWithIssues() != 2 {
		t.Errorf("RulesWithIssues() = %d, want 2", result.RulesWithIssues())
	}
}

func createTestConfig(t *testing.T) *config.Config {
	t.Helper()

	cfg := &config.Config{
		LogSources:      []string{"/tmp"},
		TimestampFormat: config.TimestampConfig{Pattern: `^(\d+)`, Layout: "2006"},
		Rules: []config.RuleConfig{{
			Name:             "test",
			Type:             "sequence",
			StartPattern:     `START id=(\w+)`,
			EndPattern:       `END id=(\w+)`,
			CorrelationField: 1,
			Timeout:          60 * time.Second,
		}},
	}

	if err := config.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	return cfg
}
