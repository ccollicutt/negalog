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

// triggerEvent tracks a trigger awaiting its expected consequence.
type triggerEvent struct {
	correlationID string // empty if no correlation field
	timestamp     time.Time
	source        string
	lineNum       int
}

// ConditionalEngine implements RuleEngine for conditional absence detection.
// It tracks trigger events and looks for expected consequences within a timeout.
type ConditionalEngine struct {
	name        string
	description string
	timeout     time.Duration
	corrField   int // 0 means no correlation, 1+ is capture group index

	triggerPattern  *regexp.Regexp
	expectedPattern *regexp.Regexp

	// State
	mu       sync.Mutex
	triggers []triggerEvent // active triggers awaiting consequence
	stats    RuleStats
}

// NewConditionalEngine creates a new conditional detection engine from a rule config.
func NewConditionalEngine(rule *config.RuleConfig) (*ConditionalEngine, error) {
	if rule.RuleTypeEnum() != config.RuleTypeConditional {
		return nil, fmt.Errorf("rule %q is not a conditional rule", rule.Name)
	}

	triggerPattern := rule.CompiledTriggerPattern()
	expectedPattern := rule.CompiledExpectedPattern()

	if triggerPattern == nil || expectedPattern == nil {
		return nil, fmt.Errorf("rule %q has uncompiled patterns", rule.Name)
	}

	return &ConditionalEngine{
		name:            rule.Name,
		description:     rule.Description,
		timeout:         rule.Timeout,
		corrField:       rule.CorrelationField,
		triggerPattern:  triggerPattern,
		expectedPattern: expectedPattern,
		triggers:        make([]triggerEvent, 0),
	}, nil
}

// Name returns the rule name.
func (e *ConditionalEngine) Name() string {
	return e.name
}

// Type returns the rule type.
func (e *ConditionalEngine) Type() RuleType {
	return RuleTypeConditional
}

// Process handles a single log line.
func (e *ConditionalEngine) Process(ctx context.Context, line *parser.ParsedLine) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.LinesProcessed++

	// Check for trigger pattern match
	if matches := e.triggerPattern.FindStringSubmatch(line.Raw); matches != nil {
		trigger := triggerEvent{
			timestamp: line.Timestamp,
			source:    line.Source,
			lineNum:   line.LineNum,
		}

		// Extract correlation ID if configured
		if e.corrField > 0 && e.corrField <= len(matches)-1 {
			trigger.correlationID = matches[e.corrField]
		}

		e.triggers = append(e.triggers, trigger)
		e.stats.LinesMatched++
	}

	// Check for expected pattern match
	if matches := e.expectedPattern.FindStringSubmatch(line.Raw); matches != nil {
		var corrID string
		if e.corrField > 0 && e.corrField <= len(matches)-1 {
			corrID = matches[e.corrField]
		}

		// Remove matching triggers that are within timeout
		e.removeSatisfiedTriggers(corrID, line.Timestamp)
	}

	return nil
}

// removeSatisfiedTriggers removes triggers that are satisfied by the expected event.
func (e *ConditionalEngine) removeSatisfiedTriggers(corrID string, eventTime time.Time) {
	newTriggers := make([]triggerEvent, 0, len(e.triggers))

	for _, trigger := range e.triggers {
		// Check if this trigger is satisfied
		satisfied := false

		// If we have correlation IDs, they must match
		if e.corrField > 0 {
			if trigger.correlationID == corrID {
				// Check timeout
				if eventTime.Sub(trigger.timestamp) <= e.timeout {
					satisfied = true
				}
			}
		} else {
			// No correlation - any expected event within timeout satisfies oldest trigger
			if eventTime.Sub(trigger.timestamp) <= e.timeout {
				satisfied = true
			}
		}

		if !satisfied {
			newTriggers = append(newTriggers, trigger)
		} else if e.corrField == 0 {
			// Without correlation, only satisfy the first matching trigger
			newTriggers = append(newTriggers, e.triggers[len(newTriggers)+1:]...)
			break
		}
	}

	e.triggers = newTriggers
}

// Finalize completes analysis and returns detected issues.
func (e *ConditionalEngine) Finalize(ctx context.Context) (*RuleResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.EndTime = time.Now()

	result := &RuleResult{
		RuleName:    e.name,
		RuleType:    RuleTypeConditional,
		Description: e.description,
		Issues:      make([]Issue, 0, len(e.triggers)),
		Stats:       e.stats,
	}

	// All remaining triggers are missing their expected consequences
	for _, trigger := range e.triggers {
		desc := fmt.Sprintf("Trigger event without expected consequence within %s", e.timeout)
		if trigger.correlationID != "" {
			desc = fmt.Sprintf("Trigger event (id=%s) without expected consequence within %s",
				trigger.correlationID, e.timeout)
		}

		issue := Issue{
			Type:        IssueTypeMissingConsequence,
			Description: desc,
			Context: IssueContext{
				CorrelationID: trigger.correlationID,
				StartTime:     trigger.timestamp,
				Source:        trigger.source,
				LineNum:       trigger.lineNum,
				Timeout:       e.timeout,
			},
		}
		result.Issues = append(result.Issues, issue)
	}

	return result, nil
}

// Reset clears internal state for reuse.
func (e *ConditionalEngine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.triggers = make([]triggerEvent, 0)
	e.stats = RuleStats{}
}

// TriggerState holds serializable state for a pending trigger.
type TriggerState struct {
	CorrelationID string    `json:"correlation_id,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source"`
	LineNum       int       `json:"line_num"`
}

// ExportState returns all pending triggers for serialization.
func (e *ConditionalEngine) ExportState() []TriggerState {
	e.mu.Lock()
	defer e.mu.Unlock()

	states := make([]TriggerState, 0, len(e.triggers))
	for _, trigger := range e.triggers {
		states = append(states, TriggerState{
			CorrelationID: trigger.correlationID,
			Timestamp:     trigger.timestamp,
			Source:        trigger.source,
			LineNum:       trigger.lineNum,
		})
	}
	return states
}

// ImportState restores pending triggers from serialized state.
func (e *ConditionalEngine) ImportState(states []TriggerState) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, s := range states {
		e.triggers = append(e.triggers, triggerEvent{
			correlationID: s.CorrelationID,
			timestamp:     s.Timestamp,
			source:        s.Source,
			lineNum:       s.LineNum,
		})
	}
}
