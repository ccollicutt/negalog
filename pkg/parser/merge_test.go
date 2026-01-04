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

func TestMergedSource_Next(t *testing.T) {
	dir := t.TempDir()

	// Create two log files with interleaved timestamps
	file1 := filepath.Join(dir, "a.log")
	file2 := filepath.Join(dir, "b.log")

	content1 := `[2024-01-15 10:00:00] A first
[2024-01-15 10:00:02] A second
[2024-01-15 10:00:04] A third
`
	content2 := `[2024-01-15 10:00:01] B first
[2024-01-15 10:00:03] B second
`

	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`)
	layout := "2006-01-02 15:04:05"

	src1 := NewFileSource([]string{file1}, pattern, layout)
	src2 := NewFileSource([]string{file2}, pattern, layout)

	merged := NewMergedSource(src1, src2)
	defer merged.Close()

	ctx := context.Background()
	var lines []*ParsedLine

	for {
		line, err := merged.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		lines = append(lines, line)
	}

	if len(lines) != 5 {
		t.Fatalf("Got %d lines, want 5", len(lines))
	}

	// Verify chronological order
	for i := 1; i < len(lines); i++ {
		if lines[i].Timestamp.Before(lines[i-1].Timestamp) {
			t.Errorf("Lines not in chronological order at index %d", i)
		}
	}

	// Verify expected order: A first, B first, A second, B second, A third
	expectedTimes := []time.Time{
		time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 15, 10, 0, 1, 0, time.UTC),
		time.Date(2024, 1, 15, 10, 0, 2, 0, time.UTC),
		time.Date(2024, 1, 15, 10, 0, 3, 0, time.UTC),
		time.Date(2024, 1, 15, 10, 0, 4, 0, time.UTC),
	}

	for i, expected := range expectedTimes {
		if !lines[i].Timestamp.Equal(expected) {
			t.Errorf("Line %d timestamp = %v, want %v", i, lines[i].Timestamp, expected)
		}
	}
}

func TestMergedSource_EmptySources(t *testing.T) {
	merged := NewMergedSource()
	defer merged.Close()

	ctx := context.Background()
	_, err := merged.Next(ctx)
	if err != io.EOF {
		t.Errorf("Next() error = %v, want io.EOF", err)
	}
}

func TestMergedSource_OneEmptySource(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.log")
	file2 := filepath.Join(dir, "b.log")

	// One file with content, one empty
	if err := os.WriteFile(file1, []byte("[2024] Line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d+)\]`)
	layout := "2006"

	src1 := NewFileSource([]string{file1}, pattern, layout)
	src2 := NewFileSource([]string{file2}, pattern, layout)

	merged := NewMergedSource(src1, src2)
	defer merged.Close()

	ctx := context.Background()
	var count int

	for {
		_, err := merged.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("Got %d lines, want 1", count)
	}
}

func TestMergedSource_Close(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.log")
	if err := os.WriteFile(file, []byte("[2024] Line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d+)\]`)
	src := NewFileSource([]string{file}, pattern, "2006")

	merged := NewMergedSource(src)

	// Read one line to initialize
	ctx := context.Background()
	_, _ = merged.Next(ctx)

	// Close should not error
	if err := merged.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestMergedSource_SingleSource(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.log")
	content := `[2024-01-15 10:00:00] First
[2024-01-15 10:00:01] Second
[2024-01-15 10:00:02] Third
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`)
	layout := "2006-01-02 15:04:05"

	src := NewFileSource([]string{file}, pattern, layout)
	merged := NewMergedSource(src)
	defer merged.Close()

	ctx := context.Background()
	var count int

	for {
		_, err := merged.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		count++
	}

	if count != 3 {
		t.Errorf("Got %d lines, want 3", count)
	}
}

func TestMergedSource_SameTimestamps(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.log")
	file2 := filepath.Join(dir, "b.log")

	// Same timestamps
	content := "[2024-01-15 10:00:00] Line\n"

	if err := os.WriteFile(file1, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`)
	layout := "2006-01-02 15:04:05"

	src1 := NewFileSource([]string{file1}, pattern, layout)
	src2 := NewFileSource([]string{file2}, pattern, layout)

	merged := NewMergedSource(src1, src2)
	defer merged.Close()

	ctx := context.Background()
	var count int

	for {
		_, err := merged.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		count++
	}

	// Should get both lines even with same timestamp
	if count != 2 {
		t.Errorf("Got %d lines, want 2", count)
	}
}
