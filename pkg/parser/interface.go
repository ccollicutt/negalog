package parser

import (
	"context"
	"io"
)

// LogSource provides an iterator over parsed log lines.
// Implementations must be safe for sequential access (not concurrent).
type LogSource interface {
	// Next returns the next parsed log line.
	// Returns io.EOF when no more lines are available.
	// Lines that cannot be parsed (e.g., no timestamp) are skipped.
	Next(ctx context.Context) (*ParsedLine, error)

	// Close releases any resources held by the source.
	Close() error
}

// Ensure io.EOF is available for callers
var _ = io.EOF
