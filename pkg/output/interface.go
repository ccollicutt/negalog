package output

import (
	"context"
	"io"
)

// Formatter renders analysis results in a specific format.
type Formatter interface {
	// Format renders the report to the given writer.
	Format(ctx context.Context, report *Report, w io.Writer) error

	// Name returns the format name (text, json).
	Name() string
}

// FormatOptions controls formatter behavior.
type FormatOptions struct {
	// Verbose enables detailed output including matched logs.
	Verbose bool

	// Quiet enables minimal summary-only output.
	Quiet bool
}
