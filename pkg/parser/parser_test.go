package parser

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestFileSource_Next(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	content := `[2024-01-15 10:00:00] First line
[2024-01-15 10:00:01] Second line
[2024-01-15 10:00:02] Third line
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`)
	layout := "2006-01-02 15:04:05"

	source := NewFileSource([]string{logFile}, pattern, layout)
	defer source.Close()

	ctx := context.Background()
	var lines []*ParsedLine

	for {
		line, err := source.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		lines = append(lines, line)
	}

	if len(lines) != 3 {
		t.Errorf("Got %d lines, want 3", len(lines))
	}

	// Check first line
	if lines[0].LineNum != 1 {
		t.Errorf("LineNum = %d, want 1", lines[0].LineNum)
	}
	if lines[0].Source != logFile {
		t.Errorf("Source = %q, want %q", lines[0].Source, logFile)
	}
	expectedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if !lines[0].Timestamp.Equal(expectedTime) {
		t.Errorf("Timestamp = %v, want %v", lines[0].Timestamp, expectedTime)
	}
}

func TestFileSource_SkipsInvalidTimestamps(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	content := `[2024-01-15 10:00:00] Valid line
No timestamp here
Also no timestamp
[2024-01-15 10:00:01] Another valid line
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`)
	layout := "2006-01-02 15:04:05"

	source := NewFileSource([]string{logFile}, pattern, layout)
	defer source.Close()

	ctx := context.Background()
	var lines []*ParsedLine

	for {
		line, err := source.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		lines = append(lines, line)
	}

	// Should only get 2 valid lines
	if len(lines) != 2 {
		t.Errorf("Got %d lines, want 2 (skipping invalid)", len(lines))
	}
}

func TestFileSource_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	files := []struct {
		name    string
		content string
	}{
		{"a.log", "[2024-01-15 10:00:00] File A\n"},
		{"b.log", "[2024-01-15 10:00:01] File B\n"},
	}

	var paths []string
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	pattern := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`)
	layout := "2006-01-02 15:04:05"

	source := NewFileSource(paths, pattern, layout)
	defer source.Close()

	ctx := context.Background()
	var lines []*ParsedLine

	for {
		line, err := source.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		lines = append(lines, line)
	}

	if len(lines) != 2 {
		t.Errorf("Got %d lines, want 2", len(lines))
	}
}

func TestFileSource_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d+)\]`)
	source := NewFileSource([]string{logFile}, pattern, "2006")
	defer source.Close()

	ctx := context.Background()
	_, err := source.Next(ctx)
	if err != io.EOF {
		t.Errorf("Next() error = %v, want io.EOF", err)
	}
}

func TestFileSource_FileNotFound(t *testing.T) {
	pattern := regexp.MustCompile(`^\[(\d+)\]`)
	source := NewFileSource([]string{"/nonexistent/file.log"}, pattern, "2006")
	defer source.Close()

	ctx := context.Background()
	_, err := source.Next(ctx)
	if err == nil {
		t.Error("Next() expected error for missing file")
	}
}

func TestFileSource_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	if err := os.WriteFile(logFile, []byte("[2024] line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d+)\]`)
	source := NewFileSource([]string{logFile}, pattern, "2006")
	defer source.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := source.Next(ctx)
	if err != context.Canceled {
		t.Errorf("Next() error = %v, want context.Canceled", err)
	}
}

func TestFileSource_Close(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	if err := os.WriteFile(logFile, []byte("[2024] line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d+)\]`)
	source := NewFileSource([]string{logFile}, pattern, "2006")

	// Read one line to open the file
	ctx := context.Background()
	_, err := source.Next(ctx)
	if err != nil && err != io.EOF {
		t.Fatalf("Next() error = %v", err)
	}

	// Close should not error
	if err := source.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
