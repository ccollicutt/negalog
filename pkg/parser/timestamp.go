package parser

import (
	"fmt"
	"regexp"
	"time"
)

// TimestampExtractor extracts and parses timestamps from log lines.
type TimestampExtractor struct {
	pattern *regexp.Regexp
	layout  string
}

// NewTimestampExtractor creates a new timestamp extractor.
func NewTimestampExtractor(pattern *regexp.Regexp, layout string) *TimestampExtractor {
	return &TimestampExtractor{
		pattern: pattern,
		layout:  layout,
	}
}

// Extract attempts to extract and parse a timestamp from a log line.
// Returns the parsed time and nil error on success.
// Returns zero time and error if the pattern doesn't match or parsing fails.
func (e *TimestampExtractor) Extract(line string) (time.Time, error) {
	matches := e.pattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return time.Time{}, fmt.Errorf("timestamp pattern did not match")
	}

	// Use the first capture group as the timestamp string
	tsStr := matches[1]

	ts, err := time.Parse(e.layout, tsStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing timestamp %q: %w", tsStr, err)
	}

	return ts, nil
}
