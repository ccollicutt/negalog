package analyzer

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"negalog/pkg/config"
	"negalog/pkg/parser"
)

// periodicMatch tracks a single match for periodic analysis.
type periodicMatch struct {
	timestamp time.Time
	source    string
	lineNum   int
}

// PeriodicEngine implements RuleEngine for periodic absence detection.
// It tracks recurring log entries and reports gaps exceeding the threshold.
type PeriodicEngine struct {
	name           string
	description    string
	maxGap         time.Duration
	minOccurrences int

	pattern *regexp.Regexp

	// State
	mu      sync.Mutex
	matches []periodicMatch
	stats   RuleStats
}

// NewPeriodicEngine creates a new periodic detection engine from a rule config.
func NewPeriodicEngine(rule *config.RuleConfig) (*PeriodicEngine, error) {
	if rule.RuleTypeEnum() != config.RuleTypePeriodic {
		return nil, fmt.Errorf("rule %q is not a periodic rule", rule.Name)
	}

	pattern := rule.CompiledPattern()
	if pattern == nil {
		return nil, fmt.Errorf("rule %q has uncompiled pattern", rule.Name)
	}

	return &PeriodicEngine{
		name:           rule.Name,
		description:    rule.Description,
		maxGap:         rule.MaxGap,
		minOccurrences: rule.MinOccurrences,
		pattern:        pattern,
		matches:        make([]periodicMatch, 0),
	}, nil
}

// Name returns the rule name.
func (e *PeriodicEngine) Name() string {
	return e.name
}

// Type returns the rule type.
func (e *PeriodicEngine) Type() RuleType {
	return RuleTypePeriodic
}

// Process handles a single log line.
func (e *PeriodicEngine) Process(ctx context.Context, line *parser.ParsedLine) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.LinesProcessed++

	if e.pattern.MatchString(line.Raw) {
		e.matches = append(e.matches, periodicMatch{
			timestamp: line.Timestamp,
			source:    line.Source,
			lineNum:   line.LineNum,
		})
		e.stats.LinesMatched++
	}

	return nil
}

// Finalize completes analysis and returns detected issues.
func (e *PeriodicEngine) Finalize(ctx context.Context) (*RuleResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.EndTime = time.Now()

	result := &RuleResult{
		RuleName:    e.name,
		RuleType:    RuleTypePeriodic,
		Description: e.description,
		Issues:      make([]Issue, 0),
		Stats:       e.stats,
	}

	// Check for gaps between consecutive matches
	for i := 1; i < len(e.matches); i++ {
		prev := e.matches[i-1]
		curr := e.matches[i]
		gap := curr.timestamp.Sub(prev.timestamp)

		if gap > e.maxGap {
			issue := Issue{
				Type: IssueTypeGapExceeded,
				Description: fmt.Sprintf("Gap of %s between occurrences (max allowed: %s)",
					gap.Round(time.Second), e.maxGap),
				Context: IssueContext{
					StartTime:   prev.timestamp,
					EndTime:     curr.timestamp,
					Source:      prev.source,
					LineNum:     prev.lineNum,
					ActualGap:   gap,
					ExpectedGap: e.maxGap,
				},
			}
			result.Issues = append(result.Issues, issue)
		}
	}

	// Check min_occurrences if specified
	if e.minOccurrences > 0 && len(e.matches) < e.minOccurrences {
		issue := Issue{
			Type: IssueTypeBelowMinOccurrences,
			Description: fmt.Sprintf("Only %d occurrences found (minimum required: %d)",
				len(e.matches), e.minOccurrences),
			Context: IssueContext{
				Occurrences: len(e.matches),
				MinRequired: e.minOccurrences,
			},
		}
		result.Issues = append(result.Issues, issue)
	}

	return result, nil
}

// Reset clears internal state for reuse.
func (e *PeriodicEngine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.matches = make([]periodicMatch, 0)
	e.stats = RuleStats{}
}

// PeriodicState holds serializable state for periodic analysis.
// Only the last match is needed for gap detection across analysis windows.
type PeriodicState struct {
	LastMatch *time.Time `json:"last_match,omitempty"`
	Source    string     `json:"source,omitempty"`
	LineNum   int        `json:"line_num,omitempty"`
}

// ExportState returns the last match time for serialization.
func (e *PeriodicEngine) ExportState() *PeriodicState {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.matches) == 0 {
		return nil
	}

	last := e.matches[len(e.matches)-1]
	return &PeriodicState{
		LastMatch: &last.timestamp,
		Source:    last.source,
		LineNum:   last.lineNum,
	}
}

// ImportState restores the last match time from serialized state.
func (e *PeriodicEngine) ImportState(state *PeriodicState) {
	if state == nil || state.LastMatch == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Insert the last match as the first match so gap detection works
	e.matches = append([]periodicMatch{{
		timestamp: *state.LastMatch,
		source:    state.Source,
		lineNum:   state.LineNum,
	}}, e.matches...)
}
