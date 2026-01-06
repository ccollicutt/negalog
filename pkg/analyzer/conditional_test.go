package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/ccollicutt/negalog/pkg/config"
	"github.com/ccollicutt/negalog/pkg/parser"
)

func TestNewConditionalEngine(t *testing.T) {
	engine := createConditionalEngine(t, 10*time.Second, 0)

	if engine.Name() != "test" {
		t.Errorf("Name() = %q, want %q", engine.Name(), "test")
	}
	if engine.Type() != RuleTypeConditional {
		t.Errorf("Type() = %v, want %v", engine.Type(), RuleTypeConditional)
	}
}

func TestNewConditionalEngine_WrongType(t *testing.T) {
	rule := &config.RuleConfig{
		Name: "test",
		Type: "sequence",
	}
	_, err := NewConditionalEngine(rule)
	if err == nil {
		t.Error("NewConditionalEngine() expected error for wrong type")
	}
}

func TestConditionalEngine_TriggerWithConsequence(t *testing.T) {
	engine := createConditionalEngine(t, 10*time.Second, 0)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Trigger
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ERROR occurred",
		Timestamp: baseTime,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Consequence within timeout
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ALERT sent",
		Timestamp: baseTime.Add(5 * time.Second),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0 (consequence found)", len(result.Issues))
	}
}

func TestConditionalEngine_TriggerWithoutConsequence(t *testing.T) {
	engine := createConditionalEngine(t, 10*time.Second, 0)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Trigger only
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ERROR occurred",
		Timestamp: baseTime,
		Source:    "test.log",
		LineNum:   1,
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
	if issue.Type != IssueTypeMissingConsequence {
		t.Errorf("Type = %v, want %v", issue.Type, IssueTypeMissingConsequence)
	}
}

func TestConditionalEngine_ConsequenceAfterTimeout(t *testing.T) {
	engine := createConditionalEngine(t, 10*time.Second, 0)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Trigger
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ERROR occurred",
		Timestamp: baseTime,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Consequence after timeout
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ALERT sent",
		Timestamp: baseTime.Add(30 * time.Second), // 30s > 10s timeout
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// Should still be missing (consequence came too late)
	if len(result.Issues) != 1 {
		t.Errorf("Issues = %d, want 1 (consequence after timeout)", len(result.Issues))
	}
}

func TestConditionalEngine_WithCorrelation(t *testing.T) {
	engine := createConditionalEngine(t, 10*time.Second, 1)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Trigger with ID=500
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ERROR code=500",
		Timestamp: baseTime,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Trigger with ID=404
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ERROR code=404",
		Timestamp: baseTime.Add(1 * time.Second),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Consequence for 500 only
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ALERT code=500",
		Timestamp: baseTime.Add(5 * time.Second),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// 404 should be missing
	if len(result.Issues) != 1 {
		t.Fatalf("Issues = %d, want 1", len(result.Issues))
	}

	if result.Issues[0].Context.CorrelationID != "404" {
		t.Errorf("CorrelationID = %q, want %q", result.Issues[0].Context.CorrelationID, "404")
	}
}

func TestConditionalEngine_MultipleTriggers(t *testing.T) {
	engine := createConditionalEngine(t, 10*time.Second, 0)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Multiple triggers
	for i := 0; i < 3; i++ {
		if err := engine.Process(ctx, &parser.ParsedLine{
			Raw:       "ERROR occurred",
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("Process() error = %v", err)
		}
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// All 3 should be missing
	if len(result.Issues) != 3 {
		t.Errorf("Issues = %d, want 3", len(result.Issues))
	}
}

func TestConditionalEngine_NoTriggers(t *testing.T) {
	engine := createConditionalEngine(t, 10*time.Second, 0)

	ctx := context.Background()

	// Non-matching line
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "INFO all good",
		Timestamp: time.Now(),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0", len(result.Issues))
	}
}

func TestConditionalEngine_Reset(t *testing.T) {
	engine := createConditionalEngine(t, 10*time.Second, 0)

	ctx := context.Background()

	// Add trigger
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "ERROR occurred",
		Timestamp: time.Now(),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Reset
	engine.Reset()

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0 after reset", len(result.Issues))
	}
}

func createConditionalEngine(t *testing.T, timeout time.Duration, corrField int) *ConditionalEngine {
	t.Helper()

	triggerPattern := `ERROR`
	expectedPattern := `ALERT`
	if corrField > 0 {
		triggerPattern = `ERROR code=(\d+)`
		expectedPattern = `ALERT code=(\d+)`
	}

	cfg := &config.Config{
		LogSources:      []string{"/tmp"},
		TimestampFormat: config.TimestampConfig{Pattern: `^(\d+)`, Layout: "2006"},
		Rules: []config.RuleConfig{{
			Name:             "test",
			Type:             "conditional",
			TriggerPattern:   triggerPattern,
			ExpectedPattern:  expectedPattern,
			CorrelationField: corrField,
			Timeout:          timeout,
		}},
	}

	if err := config.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	engine, err := NewConditionalEngine(&cfg.Rules[0])
	if err != nil {
		t.Fatalf("NewConditionalEngine() error = %v", err)
	}

	return engine
}
