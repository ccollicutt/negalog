package parser

import (
	"fmt"
	"path/filepath"
	"sort"
)

// ExpandGlobs expands a list of file paths and glob patterns into a deduplicated
// list of matching file paths. Patterns that don't match any files are returned as-is
// (the caller should handle file-not-found errors).
func ExpandGlobs(patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var result []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}

		if len(matches) == 0 {
			// Pattern didn't match anything - include it as literal path
			// This allows for explicit file paths and better error messages later
			if !seen[pattern] {
				seen[pattern] = true
				result = append(result, pattern)
			}
			continue
		}

		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				result = append(result, match)
			}
		}
	}

	// Sort for deterministic ordering
	sort.Strings(result)

	return result, nil
}
