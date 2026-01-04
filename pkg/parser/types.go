// Package parser provides log file reading and parsing functionality.
package parser

import "time"

// ParsedLine represents a single log line with extracted metadata.
type ParsedLine struct {
	// Raw is the original line content.
	Raw string

	// Timestamp is the parsed timestamp from the log line.
	Timestamp time.Time

	// Source is the file path this line came from.
	Source string

	// LineNum is the 1-based line number in the source file.
	LineNum int
}

// LogLine is a raw log line before timestamp parsing.
type LogLine struct {
	// Content is the raw line text.
	Content string

	// Source is the file path this line came from.
	Source string

	// LineNum is the 1-based line number in the source file.
	LineNum int
}
