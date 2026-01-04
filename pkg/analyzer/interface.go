package analyzer

import (
	"context"

	"negalog/pkg/parser"
)

// RuleEngine processes log lines and detects missing log patterns.
// Each detection strategy (sequence, periodic, conditional) implements this interface.
type RuleEngine interface {
	// Name returns the rule name for reporting.
	Name() string

	// Type returns the rule type (sequence, periodic, conditional).
	Type() RuleType

	// Process handles a single log line, updating internal state.
	// Returns nil on success, error on fatal problems.
	Process(ctx context.Context, line *parser.ParsedLine) error

	// Finalize completes analysis and returns detected issues.
	// Called after all log lines have been processed.
	Finalize(ctx context.Context) (*RuleResult, error)

	// Reset clears internal state for reuse.
	Reset()
}
