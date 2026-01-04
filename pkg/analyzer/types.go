// Package analyzer provides log analysis engines for detecting missing logs.
package analyzer

import (
	"time"
)

// RuleType enumerates detection strategies.
type RuleType string

const (
	RuleTypeSequence    RuleType = "sequence"
	RuleTypePeriodic    RuleType = "periodic"
	RuleTypeConditional RuleType = "conditional"
)

// IssueType categorizes detected issues.
type IssueType string

const (
	// IssueTypeMissingEnd indicates a sequence start without matching end.
	IssueTypeMissingEnd IssueType = "missing_end"

	// IssueTypeGapExceeded indicates a periodic log gap exceeds the threshold.
	IssueTypeGapExceeded IssueType = "gap_exceeded"

	// IssueTypeMissingConsequence indicates a trigger without expected consequence.
	IssueTypeMissingConsequence IssueType = "missing_consequence"

	// IssueTypeBelowMinOccurrences indicates fewer occurrences than required.
	IssueTypeBelowMinOccurrences IssueType = "below_min_occurrences"
)

// RuleResult contains findings from executing a single rule.
type RuleResult struct {
	// RuleName is the name of the rule that produced these results.
	RuleName string

	// RuleType indicates the type of detection strategy used.
	RuleType RuleType

	// Description is the rule's description, if any.
	Description string

	// Issues contains all detected problems.
	Issues []Issue

	// Stats provides execution statistics.
	Stats RuleStats
}

// RuleStats contains execution statistics for a rule.
type RuleStats struct {
	// LinesProcessed is the total number of log lines examined.
	LinesProcessed int

	// LinesMatched is the number of lines that matched the rule's patterns.
	LinesMatched int

	// StartTime is when rule processing began.
	StartTime time.Time

	// EndTime is when rule processing completed.
	EndTime time.Time
}

// HasIssues returns true if any issues were detected.
func (r *RuleResult) HasIssues() bool {
	return len(r.Issues) > 0
}

// Issue represents a single detected problem.
type Issue struct {
	// Type categorizes the issue.
	Type IssueType

	// Description is a human-readable summary of the issue.
	Description string

	// Context provides details about where/when the issue occurred.
	Context IssueContext
}

// IssueContext provides detailed information about an issue.
type IssueContext struct {
	// CorrelationID is the extracted correlation identifier, if any.
	CorrelationID string

	// StartTime is when the triggering event occurred.
	StartTime time.Time

	// EndTime is when the expected event should have occurred (zero if missing).
	EndTime time.Time

	// Source is the log file where the issue was detected.
	Source string

	// LineNum is the line number of the triggering event.
	LineNum int

	// Timeout is the expected maximum time for completion.
	Timeout time.Duration

	// ActualGap is the actual gap duration (for periodic rules).
	ActualGap time.Duration

	// ExpectedGap is the maximum allowed gap (for periodic rules).
	ExpectedGap time.Duration

	// Occurrences is the actual count (for min_occurrences checks).
	Occurrences int

	// MinRequired is the minimum required count.
	MinRequired int
}
