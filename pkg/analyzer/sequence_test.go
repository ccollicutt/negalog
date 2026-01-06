package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/ccollicutt/negalog/pkg/config"
	"github.com/ccollicutt/negalog/pkg/parser"
)

func TestNewSequenceEngine(t *testing.T) {
	rule := &config.RuleConfig{
		Name:             "test",
		Type:             "sequence",
		StartPattern:     `START id=(\w+)`,
		EndPattern:       `END id=(\w+)`,
		CorrelationField: 1,
		Timeout:          30 * time.Second,
	}

	// Validate to compile patterns
	cfg := &config.Config{
		LogSources:      []string{"/tmp"},
		TimestampFormat: config.TimestampConfig{Pattern: `^(\d+)`, Layout: "2006"},
		Rules:           []config.RuleConfig{*rule},
	}
	if err := config.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	engine, err := NewSequenceEngine(&cfg.Rules[0])
	if err != nil {
		t.Fatalf("NewSequenceEngine() error = %v", err)
	}

	if engine.Name() != "test" {
		t.Errorf("Name() = %q, want %q", engine.Name(), "test")
	}
	if engine.Type() != RuleTypeSequence {
		t.Errorf("Type() = %v, want %v", engine.Type(), RuleTypeSequence)
	}
}

func TestNewSequenceEngine_WrongType(t *testing.T) {
	rule := &config.RuleConfig{
		Name: "test",
		Type: "periodic",
	}
	_, err := NewSequenceEngine(rule)
	if err == nil {
		t.Error("NewSequenceEngine() expected error for wrong type")
	}
}

func TestSequenceEngine_CompleteSequence(t *testing.T) {
	engine := createSequenceEngine(t, 60*time.Second)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Start event
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "START id=abc123",
		Timestamp: baseTime,
		Source:    "test.log",
		LineNum:   1,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// End event within timeout
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "END id=abc123",
		Timestamp: baseTime.Add(30 * time.Second),
		Source:    "test.log",
		LineNum:   2,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0 (sequence completed)", len(result.Issues))
	}
}

func TestSequenceEngine_MissingEnd(t *testing.T) {
	engine := createSequenceEngine(t, 60*time.Second)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Start event only
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "START id=abc123",
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
	if issue.Type != IssueTypeMissingEnd {
		t.Errorf("Type = %v, want %v", issue.Type, IssueTypeMissingEnd)
	}
	if issue.Context.CorrelationID != "abc123" {
		t.Errorf("CorrelationID = %q, want %q", issue.Context.CorrelationID, "abc123")
	}
}

func TestSequenceEngine_MultipleSequences(t *testing.T) {
	engine := createSequenceEngine(t, 60*time.Second)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Three starts, only one end
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "START id=a",
		Timestamp: baseTime,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "START id=b",
		Timestamp: baseTime.Add(1 * time.Second),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "START id=c",
		Timestamp: baseTime.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "END id=b",
		Timestamp: baseTime.Add(10 * time.Second),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// a and c should be missing
	if len(result.Issues) != 2 {
		t.Errorf("Issues = %d, want 2", len(result.Issues))
	}
}

func TestSequenceEngine_EndAfterTimeout(t *testing.T) {
	engine := createSequenceEngine(t, 10*time.Second)

	ctx := context.Background()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Start
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "START id=abc",
		Timestamp: baseTime,
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// End after timeout
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "END id=abc",
		Timestamp: baseTime.Add(30 * time.Second), // 30s > 10s timeout
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// Should still be missing (end came after timeout)
	if len(result.Issues) != 1 {
		t.Errorf("Issues = %d, want 1 (end after timeout)", len(result.Issues))
	}
}

func TestSequenceEngine_NoMatch(t *testing.T) {
	engine := createSequenceEngine(t, 60*time.Second)

	ctx := context.Background()

	// Line that doesn't match
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

	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0", len(result.Issues))
	}
}

func TestSequenceEngine_Reset(t *testing.T) {
	engine := createSequenceEngine(t, 60*time.Second)

	ctx := context.Background()

	// Add a start
	if err := engine.Process(ctx, &parser.ParsedLine{
		Raw:       "START id=abc",
		Timestamp: time.Now(),
	}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Reset
	engine.Reset()

	// Finalize should have no issues
	result, err := engine.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Issues = %d, want 0 after reset", len(result.Issues))
	}
}

func createSequenceEngine(t *testing.T, timeout time.Duration) *SequenceEngine {
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
			Timeout:          timeout,
		}},
	}

	if err := config.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	engine, err := NewSequenceEngine(&cfg.Rules[0])
	if err != nil {
		t.Fatalf("NewSequenceEngine() error = %v", err)
	}

	return engine
}
