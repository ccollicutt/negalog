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

// sequenceTracker tracks an open sequence awaiting completion.
type sequenceTracker struct {
	correlationID string
	startTime     time.Time
	source        string
	lineNum       int
}

// SequenceEngine implements RuleEngine for sequence gap detection.
// It tracks start events and looks for matching end events within a timeout.
type SequenceEngine struct {
	name        string
	description string
	timeout     time.Duration
	corrField   int // 1-based capture group index

	startPattern *regexp.Regexp
	endPattern   *regexp.Regexp

	// State
	mu            sync.Mutex
	openSequences map[string]*sequenceTracker // key: correlation ID
	stats         RuleStats
}

// NewSequenceEngine creates a new sequence detection engine from a rule config.
func NewSequenceEngine(rule *config.RuleConfig) (*SequenceEngine, error) {
	if rule.RuleTypeEnum() != config.RuleTypeSequence {
		return nil, fmt.Errorf("rule %q is not a sequence rule", rule.Name)
	}

	startPattern := rule.CompiledStartPattern()
	endPattern := rule.CompiledEndPattern()

	if startPattern == nil || endPattern == nil {
		return nil, fmt.Errorf("rule %q has uncompiled patterns", rule.Name)
	}

	return &SequenceEngine{
		name:          rule.Name,
		description:   rule.Description,
		timeout:       rule.Timeout,
		corrField:     rule.CorrelationField,
		startPattern:  startPattern,
		endPattern:    endPattern,
		openSequences: make(map[string]*sequenceTracker),
	}, nil
}

// Name returns the rule name.
func (e *SequenceEngine) Name() string {
	return e.name
}

// Type returns the rule type.
func (e *SequenceEngine) Type() RuleType {
	return RuleTypeSequence
}

// Process handles a single log line.
func (e *SequenceEngine) Process(ctx context.Context, line *parser.ParsedLine) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.LinesProcessed++

	// Check for start pattern match
	if matches := e.startPattern.FindStringSubmatch(line.Raw); matches != nil {
		if e.corrField <= len(matches)-1 {
			corrID := matches[e.corrField]
			e.openSequences[corrID] = &sequenceTracker{
				correlationID: corrID,
				startTime:     line.Timestamp,
				source:        line.Source,
				lineNum:       line.LineNum,
			}
			e.stats.LinesMatched++
		}
	}

	// Check for end pattern match
	if matches := e.endPattern.FindStringSubmatch(line.Raw); matches != nil {
		if e.corrField <= len(matches)-1 {
			corrID := matches[e.corrField]
			if tracker, exists := e.openSequences[corrID]; exists {
				// Check if within timeout
				elapsed := line.Timestamp.Sub(tracker.startTime)
				if elapsed <= e.timeout {
					// Successfully completed within timeout
					delete(e.openSequences, corrID)
				}
				// If elapsed > timeout, leave it open so it's reported as missing
			}
		}
	}

	return nil
}

// Finalize completes analysis and returns detected issues.
func (e *SequenceEngine) Finalize(ctx context.Context) (*RuleResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.EndTime = time.Now()

	result := &RuleResult{
		RuleName:    e.name,
		RuleType:    RuleTypeSequence,
		Description: e.description,
		Issues:      make([]Issue, 0, len(e.openSequences)),
		Stats:       e.stats,
	}

	// All remaining open sequences are missing their end events
	for _, tracker := range e.openSequences {
		issue := Issue{
			Type: IssueTypeMissingEnd,
			Description: fmt.Sprintf("Sequence started but not completed within %s",
				e.timeout),
			Context: IssueContext{
				CorrelationID: tracker.correlationID,
				StartTime:     tracker.startTime,
				Source:        tracker.source,
				LineNum:       tracker.lineNum,
				Timeout:       e.timeout,
			},
		}
		result.Issues = append(result.Issues, issue)
	}

	return result, nil
}

// Reset clears internal state for reuse.
func (e *SequenceEngine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.openSequences = make(map[string]*sequenceTracker)
	e.stats = RuleStats{}
}

// SequenceState holds serializable state for a pending sequence.
type SequenceState struct {
	CorrelationID string    `json:"correlation_id"`
	StartTime     time.Time `json:"start_time"`
	Source        string    `json:"source"`
	LineNum       int       `json:"line_num"`
}

// ExportState returns all pending sequences for serialization.
func (e *SequenceEngine) ExportState() []SequenceState {
	e.mu.Lock()
	defer e.mu.Unlock()

	states := make([]SequenceState, 0, len(e.openSequences))
	for _, tracker := range e.openSequences {
		states = append(states, SequenceState{
			CorrelationID: tracker.correlationID,
			StartTime:     tracker.startTime,
			Source:        tracker.source,
			LineNum:       tracker.lineNum,
		})
	}
	return states
}

// ImportState restores pending sequences from serialized state.
func (e *SequenceEngine) ImportState(states []SequenceState) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, s := range states {
		e.openSequences[s.CorrelationID] = &sequenceTracker{
			correlationID: s.CorrelationID,
			startTime:     s.StartTime,
			source:        s.Source,
			lineNum:       s.LineNum,
		}
	}
}
