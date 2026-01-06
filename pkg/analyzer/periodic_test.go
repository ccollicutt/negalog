package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/ccollicutt/negalog/pkg/config"
	"github.com/ccollicutt/negalog/pkg/parser"
)

func TestNewPeriodicEngine(t *testing.T) {
	engine := createPeriodicEngine(t, 5*time.Minute, 0)

	if engine.Name() != "test" {
		t.Errorf("Name() = %q, want %q", engine.Name(), "test")
	}
	if engine.Type() != RuleTypePeriodic {
		t.Errorf("Type() = %v, want %v", engine.Type(), RuleTypePeriodic)
	}
}

func TestNewPeriodicEngine_WrongType(t *testing.T) {
	rule := &config.RuleConfig{
		Name: "test",
		Type: "sequence",
	}
	_, err := NewPeriodicEngine(rule)
	if err == nil {
		t.Error("NewPeriodicEngine() expected error for wrong type")
	}
}

func TestPeriodicEngine_NoGaps(t *testing.T) {
	engine := createPeriodicEngine(t, 5*time.Minute, 0)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Regular heartbeats every 2 minutes (within 5 minute max gap)
	for i := 0; i < 5; i++ {
		if err := engine.Process(ctx, &parser.ParsedLine{
			Raw:       "HEARTBEAT",
			Timestamp: baseTime.Add(time.Duration(i) * 2 * time.Minute),
		}); err != nil {
			t.Fatalf("Process() error = %v", err)
		}
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0 (no gaps)", len(result.Issues))
	}
}

func TestPeriodicEngine_GapExceeded(t *testing.T) {
	engine := createPeriodicEngine(t, 5*time.Minute, 0)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Heartbeats with a gap
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "HEARTBEAT",
		Timestamp: baseTime,
		Source:    "test.log",
		LineNum:   1,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "HEARTBEAT",
		Timestamp: baseTime.Add(10 * time.Minute), // 10m > 5m max gap
		Source:    "test.log",
		LineNum:   2,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("Issues = %d, want 1", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Type != IssueTypeGapExceeded {
		t.Errorf("Type = %v, want %v", issue.Type, IssueTypeGapExceeded)
	}
	if issue.Context.ActualGap != 10*time.Minute {
		t.Errorf("ActualGap = %v, want 10m", issue.Context.ActualGap)
	}
}

func TestPeriodicEngine_MultipleGaps(t *testing.T) {
	engine := createPeriodicEngine(t, 5*time.Minute, 0)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Pattern: OK, gap, OK, gap, OK
	times := []time.Duration{0, 3 * time.Minute, 15 * time.Minute, 18 * time.Minute, 30 * time.Minute}
	for _, d := range times {
		if err := engine.Process(ctx, &parser.ParsedLine{
			Raw:       "HEARTBEAT",
			Timestamp: baseTime.Add(d),
		}); err != nil {
			t.Fatalf("Process() error = %v", err)
		}
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// Gaps: 3m (OK), 12m (BAD), 3m (OK), 12m (BAD)
	if len(result.Issues) != 2 {
		t.Errorf("Issues = %d, want 2", len(result.Issues))
	}
}

func TestPeriodicEngine_MinOccurrences(t *testing.T) {
	engine := createPeriodicEngine(t, 5*time.Minute, 10)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Only 3 occurrences (less than min 10)
	for i := 0; i < 3; i++ {
		if err := engine.Process(ctx, &parser.ParsedLine{
			Raw:       "HEARTBEAT",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("Process() error = %v", err)
		}
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// Should have one issue for below min occurrences
	found := false
	for _, issue := range result.Issues {
		if issue.Type == IssueTypeBelowMinOccurrences {
			found = true
			if issue.Context.Occurrences != 3 {
				t.Errorf("Occurrences = %d, want 3", issue.Context.Occurrences)
			}
			if issue.Context.MinRequired != 10 {
				t.Errorf("MinRequired = %d, want 10", issue.Context.MinRequired)
			}
		}
	}
	if !found {
		t.Error("Expected IssueTypeBelowMinOccurrences issue")
	}
}

func TestPeriodicEngine_NoMatches(t *testing.T) {
	engine := createPeriodicEngine(t, 5*time.Minute, 0)

	ctx := context.Background()

	// Non-matching lines
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "Something else",
		Timestamp: time.Now(),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// No issues (no gaps to report with 0 or 1 matches)
	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0", len(result.Issues))
	}
}

func TestPeriodicEngine_SingleMatch(t *testing.T) {
	engine := createPeriodicEngine(t, 5*time.Minute, 0)

	ctx := context.Background()

	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "HEARTBEAT",
		Timestamp: time.Now(),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// Single match can't have gaps
	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0", len(result.Issues))
	}
}

func TestPeriodicEngine_Reset(t *testing.T) {
	engine := createPeriodicEngine(t, 1*time.Minute, 0)

	ctx := context.Background()
	baseTime := time.Now()

	// Add matches with gap
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "HEARTBEAT",
		Timestamp: baseTime,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "HEARTBEAT",
		Timestamp: baseTime.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Reset
	engine.Reset()

	// Should have no issues
	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0 after reset", len(result.Issues))
	}
}

func createPeriodicEngine(t *testing.T, maxGap time.Duration, minOccurrences int) *PeriodicEngine {
	t.Helper()

	cfg := &config.Config{
		LogSources:      []string{"/tmp"},
		TimestampFormat: config.TimestampConfig{Pattern: `^(\d+)`, Layout: "2006"},
		Rules: []config.RuleConfig{{
			Name:           "test",
			Type:           "periodic",
			Pattern:        `HEARTBEAT`,
			MaxGap:         maxGap,
			MinOccurrences: minOccurrences,
		}},
	}

	if err := config.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	engine, err := NewPeriodicEngine(&cfg.Rules[0])
	if err != nil {
		t.Fatalf("NewPeriodicEngine() error = %v", err)
	}

	return engine
}
