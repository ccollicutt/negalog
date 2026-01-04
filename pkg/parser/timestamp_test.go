package parser

import (
	"regexp"
	"testing"
	"time"
)

func TestTimestampExtractor_Extract(t *testing.T) {
	pattern := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`)
	layout := "2006-01-02 15:04:05"
	extractor := NewTimestampExtractor(pattern, layout)

	tests := []struct {
		name    string
		line    string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "valid timestamp",
			line:    "[2024-01-15 10:30:00] Some log message",
			want:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "no match",
			line:    "No timestamp here",
			wantErr: true,
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "partial match",
			line:    "[2024-01-15",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractor.Extract(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("Extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("Extract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimestampExtractor_DifferentFormats(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		layout  string
		line    string
		want    time.Time
	}{
		{
			name:    "ISO format",
			pattern: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})`,
			layout:  "2006-01-02T15:04:05",
			line:    "2024-01-15T10:30:00 message",
			want:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:    "Unix-style syslog",
			pattern: `^(\w{3}\s+\d+\s+\d{2}:\d{2}:\d{2})`,
			layout:  "Jan  2 15:04:05",
			line:    "Jan 15 10:30:00 hostname message",
			want:    time.Date(0, 1, 15, 10, 30, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := regexp.MustCompile(tt.pattern)
			extractor := NewTimestampExtractor(pattern, tt.layout)
			got, err := extractor.Extract(tt.line)
			if err != nil {
				t.Fatalf("Extract() error = %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("Extract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTimestampExtractor(t *testing.T) {
	pattern := regexp.MustCompile(`^(\d+)`)
	layout := "2006"
	extractor := NewTimestampExtractor(pattern, layout)

	if extractor == nil {
		t.Fatal("NewTimestampExtractor() returned nil")
	}
}
