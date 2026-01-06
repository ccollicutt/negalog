// Package output provides formatting and output generation for analysis results.
package output

import (
	"time"

	"github.com/ccollicutt/negalog/pkg/analyzer"
)

// Report is the complete analysis output.
type Report struct {
	// Summary provides aggregate statistics.
	Summary Summary

	// Results contains findings from each rule.
	Results []*analyzer.RuleResult

	// Metadata provides context about the analysis.
	Metadata Metadata
}

// Summary provides aggregate statistics.
type Summary struct {
	// RulesChecked is the number of rules that were executed.
	RulesChecked int

	// RulesWithIssues is the number of rules that detected issues.
	RulesWithIssues int

	// TotalIssues is the total number of issues detected.
	TotalIssues int

	// LinesProcessed is the total number of log lines analyzed.
	LinesProcessed int
}

// Metadata provides context about the analysis run.
type Metadata struct {
	// ConfigFile is the path to the configuration file used.
	ConfigFile string

	// Sources lists the log files that were analyzed.
	Sources []string

	// TimeRange is the time filter that was applied, if any.
	TimeRange *TimeRange

	// AnalyzedAt is when the analysis was performed.
	AnalyzedAt time.Time

	// Duration is how long the analysis took.
	Duration time.Duration
}

// TimeRange represents a time window for filtering.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// NewReport creates a Report from analysis results.
func NewReport(result *analyzer.AnalysisResult, configFile string) *Report {
	report := &Report{
		Results: result.Results,
		Metadata: Metadata{
			ConfigFile: configFile,
			Sources:    result.Metadata.Sources,
			AnalyzedAt: result.Metadata.EndTime,
			Duration:   result.Metadata.EndTime.Sub(result.Metadata.StartTime),
		},
		Summary: Summary{
			RulesChecked:    len(result.Results),
			RulesWithIssues: result.RulesWithIssues(),
			TotalIssues:     result.TotalIssues(),
			LinesProcessed:  result.Metadata.LinesProcessed,
		},
	}

	if result.Metadata.TimeRange != nil {
		report.Metadata.TimeRange = &TimeRange{
			Start: result.Metadata.TimeRange.Start,
			End:   result.Metadata.TimeRange.End,
		}
	}

	return report
}

// HasIssues returns true if any issues were detected.
func (r *Report) HasIssues() bool {
	return r.Summary.TotalIssues > 0
}
