// Package detector provides automatic timestamp format detection for log files.
package detector

import (
	"bufio"
	"context"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DetectionResult holds the result of analyzing a log file.
type DetectionResult struct {
	Matches       []FormatMatch // Formats that matched, sorted by confidence descending
	SampledLines  int           // Number of lines sampled
	ParsedLines   int           // Number of lines with detected timestamps
	AmbiguityNote string        // Warning about date ordering if applicable
}

// FormatMatch represents a format that matched with its confidence score.
type FormatMatch struct {
	Format     *TimestampFormat
	Confidence float64   // 0.0 to 1.0 (percentage of lines matched)
	MatchCount int       // Number of lines that matched
	SampleLine string    // Example line that matched
	ParsedTime time.Time // Parsed timestamp from sample
}

// Detector analyzes log files to identify timestamp formats.
type Detector struct {
	formats    []*TimestampFormat
	sampleSize int
}

// Option configures the Detector.
type Option func(*Detector)

// WithSampleSize sets the number of lines to sample (default 100).
func WithSampleSize(n int) Option {
	return func(d *Detector) {
		if n > 0 {
			d.sampleSize = n
		}
	}
}

// New creates a new Detector with default formats.
func New(opts ...Option) *Detector {
	d := &Detector{
		formats:    DefaultFormats(),
		sampleSize: 100,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// DetectFromFile analyzes a log file and returns detected formats.
func (d *Detector) DetectFromFile(ctx context.Context, path string) (*DetectionResult, error) {
	lines, err := d.sampleFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return d.DetectFromLines(lines), nil
}

// DetectFromLines analyzes a slice of log lines.
func (d *Detector) DetectFromLines(lines []string) *DetectionResult {
	result := &DetectionResult{
		SampledLines: len(lines),
	}

	if len(lines) == 0 {
		return result
	}

	// Track matches per format
	type formatStats struct {
		format     *TimestampFormat
		matchCount int
		sampleLine string
		parsedTime time.Time
	}

	stats := make(map[string]*formatStats)

	// Test each line against all formats
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		for _, format := range d.formats {
			matches := format.Pattern.FindStringSubmatch(line)
			if len(matches) < 2 {
				continue
			}

			tsStr := matches[1]
			parsedTime, ok := d.parseTimestamp(tsStr, format.Layout)
			if !ok {
				continue
			}

			// Track this match
			key := format.Name
			if stats[key] == nil {
				stats[key] = &formatStats{
					format:     format,
					sampleLine: line,
					parsedTime: parsedTime,
				}
			}
			stats[key].matchCount++
		}
	}

	// Convert to FormatMatch slice
	for _, s := range stats {
		result.Matches = append(result.Matches, FormatMatch{
			Format:     s.format,
			Confidence: float64(s.matchCount) / float64(len(lines)),
			MatchCount: s.matchCount,
			SampleLine: s.sampleLine,
			ParsedTime: s.parsedTime,
		})
	}

	// Sort by confidence descending, then by pattern length (more specific first)
	sort.Slice(result.Matches, func(i, j int) bool {
		if result.Matches[i].Confidence != result.Matches[j].Confidence {
			return result.Matches[i].Confidence > result.Matches[j].Confidence
		}
		// For same confidence, prefer longer patterns (more specific)
		return len(result.Matches[i].Format.PatternStr) > len(result.Matches[j].Format.PatternStr)
	})

	// Calculate total parsed lines (using best match)
	if len(result.Matches) > 0 {
		result.ParsedLines = result.Matches[0].MatchCount
	}

	// Check for ambiguity in top match
	if len(result.Matches) > 0 && result.Matches[0].Format.Ambiguous {
		result.AmbiguityNote = "This format has date ordering ambiguity (MM/DD vs DD/MM). " +
			"Verify the layout matches your log format. " +
			"For European format (DD/MM/YYYY), use layout: \"02/01/2006 15:04:05\""
	}

	return result
}

// parseTimestamp parses a timestamp string using the given layout.
// Handles special cases like Unix timestamps.
func (d *Detector) parseTimestamp(tsStr, layout string) (time.Time, bool) {
	switch layout {
	case "UNIX_SECONDS":
		secs, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		// Sanity check: reasonable Unix timestamp range (1970-2100)
		if secs < 0 || secs > 4102444800 {
			return time.Time{}, false
		}
		return time.Unix(secs, 0), true

	case "UNIX_MILLIS":
		millis, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		secs := millis / 1000
		// Sanity check: reasonable Unix timestamp range (1970-2100)
		if secs < 0 || secs > 4102444800 {
			return time.Time{}, false
		}
		return time.UnixMilli(millis), true

	default:
		t, err := time.Parse(layout, tsStr)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}
}

// sampleFile reads up to sampleSize lines from a file.
// Uses simple head sampling for efficiency.
func (d *Detector) sampleFile(_ context.Context, path string) ([]string, error) {
	// #nosec G304 - path is provided by user via CLI
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() && len(lines) < d.sampleSize {
		line := scanner.Text()
		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// BestMatch returns the highest confidence match, or nil if none found.
func (r *DetectionResult) BestMatch() *FormatMatch {
	if len(r.Matches) == 0 {
		return nil
	}
	return &r.Matches[0]
}

// HasMatch returns true if at least one format matched.
func (r *DetectionResult) HasMatch() bool {
	return len(r.Matches) > 0
}
