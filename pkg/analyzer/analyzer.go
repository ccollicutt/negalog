package analyzer

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ccollicutt/negalog/pkg/config"
	"github.com/ccollicutt/negalog/pkg/parser"
)

// Analyzer orchestrates log analysis across multiple rules.
type Analyzer struct {
	cfg     *config.Config
	engines []RuleEngine

	// Options
	timeRange  *TimeRange
	ruleFilter map[string]bool // nil means all rules
	verbose    bool
}

// TimeRange defines a time window for filtering log lines.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// AnalyzerOption configures analyzer behavior.
type AnalyzerOption func(*Analyzer)

// WithTimeRange limits analysis to logs within the given time range.
func WithTimeRange(start, end time.Time) AnalyzerOption {
	return func(a *Analyzer) {
		a.timeRange = &TimeRange{Start: start, End: end}
	}
}

// WithRuleFilter limits analysis to the specified rules.
func WithRuleFilter(rules []string) AnalyzerOption {
	return func(a *Analyzer) {
		if len(rules) > 0 {
			a.ruleFilter = make(map[string]bool)
			for _, r := range rules {
				a.ruleFilter[r] = true
			}
		}
	}
}

// WithVerbose enables verbose output.
func WithVerbose(v bool) AnalyzerOption {
	return func(a *Analyzer) {
		a.verbose = v
	}
}

// NewAnalyzer creates a new analyzer from configuration.
func NewAnalyzer(cfg *config.Config, opts ...AnalyzerOption) (*Analyzer, error) {
	a := &Analyzer{
		cfg:     cfg,
		engines: make([]RuleEngine, 0, len(cfg.Rules)),
	}

	// Apply options
	for _, opt := range opts {
		opt(a)
	}

	// Create engines for each rule
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]

		// Skip if rule filter is active and this rule isn't included
		if a.ruleFilter != nil && !a.ruleFilter[rule.Name] {
			continue
		}

		engine, err := createEngine(rule)
		if err != nil {
			return nil, fmt.Errorf("creating engine for rule %q: %w", rule.Name, err)
		}
		a.engines = append(a.engines, engine)
	}

	if len(a.engines) == 0 {
		return nil, fmt.Errorf("no rules to execute (check --rule filter)")
	}

	return a, nil
}

// createEngine creates the appropriate rule engine based on rule type.
func createEngine(rule *config.RuleConfig) (RuleEngine, error) {
	switch rule.RuleTypeEnum() {
	case config.RuleTypeSequence:
		return NewSequenceEngine(rule)
	case config.RuleTypePeriodic:
		return NewPeriodicEngine(rule)
	case config.RuleTypeConditional:
		return NewConditionalEngine(rule)
	default:
		return nil, fmt.Errorf("unknown rule type: %s", rule.Type)
	}
}

// AnalysisResult contains the complete analysis output.
type AnalysisResult struct {
	// Results contains findings from each rule.
	Results []*RuleResult

	// Metadata provides context about the analysis.
	Metadata AnalysisMetadata
}

// AnalysisMetadata provides context about the analysis run.
type AnalysisMetadata struct {
	// ConfigFile is the path to the configuration file used.
	ConfigFile string

	// Sources lists the log files that were analyzed.
	Sources []string

	// TimeRange is the time filter applied, if any.
	TimeRange *TimeRange

	// StartTime is when analysis began.
	StartTime time.Time

	// EndTime is when analysis completed.
	EndTime time.Time

	// LinesProcessed is the total number of log lines examined.
	LinesProcessed int
}

// TotalIssues returns the total number of issues across all rules.
func (r *AnalysisResult) TotalIssues() int {
	total := 0
	for _, result := range r.Results {
		total += len(result.Issues)
	}
	return total
}

// RulesWithIssues returns the count of rules that detected issues.
func (r *AnalysisResult) RulesWithIssues() int {
	count := 0
	for _, result := range r.Results {
		if result.HasIssues() {
			count++
		}
	}
	return count
}

// Analyze processes log files and returns analysis results.
func (a *Analyzer) Analyze(ctx context.Context, source parser.LogSource) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Results: make([]*RuleResult, 0, len(a.engines)),
		Metadata: AnalysisMetadata{
			TimeRange: a.timeRange,
			StartTime: time.Now(),
		},
	}

	// Reset all engines before analysis
	for _, engine := range a.engines {
		engine.Reset()
	}

	// Track sources seen
	sourcesMap := make(map[string]bool)

	// Process all log lines
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line, err := source.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading log source: %w", err)
		}

		// Track source files
		if !sourcesMap[line.Source] {
			sourcesMap[line.Source] = true
			result.Metadata.Sources = append(result.Metadata.Sources, line.Source)
		}

		// Apply time range filter
		if a.timeRange != nil {
			if line.Timestamp.Before(a.timeRange.Start) || line.Timestamp.After(a.timeRange.End) {
				continue
			}
		}

		result.Metadata.LinesProcessed++

		// Process line through all engines
		for _, engine := range a.engines {
			if err := engine.Process(ctx, line); err != nil {
				return nil, fmt.Errorf("processing line with rule %q: %w", engine.Name(), err)
			}
		}
	}

	// Finalize all engines
	for _, engine := range a.engines {
		ruleResult, err := engine.Finalize(ctx)
		if err != nil {
			return nil, fmt.Errorf("finalizing rule %q: %w", engine.Name(), err)
		}
		result.Results = append(result.Results, ruleResult)
	}

	result.Metadata.EndTime = time.Now()

	return result, nil
}
