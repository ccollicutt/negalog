package output

import (
	"context"
	"encoding/json"
	"io"
)

// JSONFormatter formats reports as JSON.
type JSONFormatter struct {
	opts FormatOptions
}

// NewJSONFormatter creates a new JSON formatter with the given options.
func NewJSONFormatter(opts FormatOptions) *JSONFormatter {
	return &JSONFormatter{opts: opts}
}

// Name returns the format name.
func (f *JSONFormatter) Name() string {
	return "json"
}

// Format renders the report as JSON.
func (f *JSONFormatter) Format(ctx context.Context, report *Report, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if f.opts.Quiet {
		// Quiet mode: just summary
		return encoder.Encode(report.Summary)
	}

	return encoder.Encode(report)
}
