package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandGlobs_SingleFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.log")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ExpandGlobs([]string{file})
	if err != nil {
		t.Fatalf("ExpandGlobs() error = %v", err)
	}
	if len(result) != 1 || result[0] != file {
		t.Errorf("ExpandGlobs() = %v, want [%s]", result, file)
	}
}

func TestExpandGlobs_GlobPattern(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.log", "b.log", "c.txt"}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	pattern := filepath.Join(dir, "*.log")
	result, err := ExpandGlobs([]string{pattern})
	if err != nil {
		t.Fatalf("ExpandGlobs() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("ExpandGlobs() returned %d files, want 2", len(result))
	}
}

func TestExpandGlobs_NoMatch(t *testing.T) {
	dir := t.TempDir()
	pattern := filepath.Join(dir, "*.nonexistent")

	result, err := ExpandGlobs([]string{pattern})
	if err != nil {
		t.Fatalf("ExpandGlobs() error = %v", err)
	}
	// Should return the pattern as-is when no match
	if len(result) != 1 || result[0] != pattern {
		t.Errorf("ExpandGlobs() = %v, want [%s]", result, pattern)
	}
}

func TestExpandGlobs_Deduplication(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.log")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Same file via different paths/patterns
	result, err := ExpandGlobs([]string{file, file})
	if err != nil {
		t.Fatalf("ExpandGlobs() error = %v", err)
	}
	if len(result) != 1 {
		t.Errorf("ExpandGlobs() returned %d files, want 1 (deduplicated)", len(result))
	}
}

func TestExpandGlobs_InvalidPattern(t *testing.T) {
	_, err := ExpandGlobs([]string{"[invalid"})
	if err == nil {
		t.Error("ExpandGlobs() expected error for invalid pattern")
	}
}

func TestExpandGlobs_MultiplePatterns(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"a.log":        dir,
		"b.log":        dir,
		"subdir/c.log": dir,
	}
	for name, base := range files {
		path := filepath.Join(base, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	patterns := []string{
		filepath.Join(dir, "*.log"),
		filepath.Join(dir, "subdir", "*.log"),
	}
	result, err := ExpandGlobs(patterns)
	if err != nil {
		t.Fatalf("ExpandGlobs() error = %v", err)
	}
	if len(result) != 3 {
		t.Errorf("ExpandGlobs() returned %d files, want 3", len(result))
	}
}

func TestExpandGlobs_Sorted(t *testing.T) {
	dir := t.TempDir()
	files := []string{"c.log", "a.log", "b.log"}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	pattern := filepath.Join(dir, "*.log")
	result, err := ExpandGlobs([]string{pattern})
	if err != nil {
		t.Fatalf("ExpandGlobs() error = %v", err)
	}

	// Should be sorted
	for i := 1; i < len(result); i++ {
		if result[i-1] > result[i] {
			t.Errorf("ExpandGlobs() result not sorted: %v", result)
			break
		}
	}
}

func TestExpandGlobs_EmptyInput(t *testing.T) {
	result, err := ExpandGlobs([]string{})
	if err != nil {
		t.Fatalf("ExpandGlobs() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("ExpandGlobs([]) = %v, want empty", result)
	}
}
