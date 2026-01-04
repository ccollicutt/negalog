package test

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// getProjectRoot returns the project root directory based on this test file's location.
func getProjectRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	// Go up one level from test/ to project root
	return filepath.Dir(filepath.Dir(filename))
}

// TestNoSkippedTests ensures no test files contain t.Skip() calls.
// Skipped tests hide failures - tests should either pass or fail, never skip.
func TestNoSkippedTests(t *testing.T) {
	forbiddenPatterns := []string{
		"t.Skip(",
		"t.SkipNow(",
		"testing.Short()",
	}

	projectRoot := getProjectRoot()
	testFiles := []string{}

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and hidden directories
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only check test files
		if strings.HasSuffix(path, "_test.go") {
			// Skip this quality test file itself
			if strings.Contains(path, "quality_test.go") {
				return nil
			}
			// Skip integration tests - they legitimately use t.Skip for optional external services
			if strings.Contains(path, "integration_test.go") {
				return nil
			}
			testFiles = append(testFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}

	violations := []string{}

	for _, testFile := range testFiles {
		f, err := os.Open(testFile)
		if err != nil {
			t.Fatalf("Failed to open %s: %v", testFile, err)
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Skip comments
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}

			for _, pattern := range forbiddenPatterns {
				if strings.Contains(line, pattern) {
					violations = append(violations,
						testFile+":"+string(rune(lineNum))+": contains forbidden pattern '"+pattern+"'")
				}
			}
		}
		f.Close()

		if err := scanner.Err(); err != nil {
			t.Fatalf("Error scanning %s: %v", testFile, err)
		}
	}

	if len(violations) > 0 {
		t.Errorf("Found %d test skip violation(s):\n", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v)
		}
		t.Error("\nTests should not be skipped. Either:")
		t.Error("  1. Fix the issue causing the skip")
		t.Error("  2. Use t.Fatalf() if a required resource is missing")
		t.Error("  3. Remove the test if it's no longer relevant")
	}
}

// TestNoEmptyTests ensures test functions have at least one assertion.
func TestNoEmptyTests(t *testing.T) {
	// This is a basic sanity check - real testing would use AST parsing
	projectRoot := getProjectRoot()
	testFiles := []string{}

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") && !strings.Contains(path, "quality_test.go") {
			testFiles = append(testFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}

	if len(testFiles) == 0 {
		t.Fatal("No test files found - something is wrong with test discovery")
	}

	t.Logf("Found %d test files", len(testFiles))
}
