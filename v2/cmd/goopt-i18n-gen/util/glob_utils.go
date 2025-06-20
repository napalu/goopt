package util

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ExpandGlobPatterns expands glob patterns, including support for ** recursive matching
func ExpandGlobPatterns(patterns []string) ([]string, error) {
	var results []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Check if pattern contains **
		if strings.Contains(pattern, "**") {
			// Handle recursive pattern
			matches, err := expandRecursivePattern(pattern)
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				if !seen[match] {
					seen[match] = true
					results = append(results, match)
				}
			}
		} else {
			// Use standard glob
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, err
			}

			// If no matches and pattern looks like a literal file path, check if it exists
			if len(matches) == 0 && !strings.ContainsAny(pattern, "*?[") {
				if _, err := os.Stat(pattern); err == nil {
					matches = []string{pattern}
				}
			}

			for _, match := range matches {
				if !seen[match] {
					seen[match] = true
					results = append(results, match)
				}
			}
		}
	}

	return results, nil
}

// expandRecursivePattern handles patterns with ** for recursive directory matching
func expandRecursivePattern(pattern string) ([]string, error) {
	// Split pattern at **
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		// For simplicity, only handle patterns with single **
		return filepath.Glob(pattern)
	}

	basePath := strings.TrimSuffix(parts[0], "/")
	if basePath == "" {
		basePath = "."
	}

	suffix := strings.TrimPrefix(parts[1], "/")

	var matches []string

	// First, check files in the base directory itself if pattern starts with **/
	if parts[0] == "" || parts[0] == "/" {
		// Pattern like **/*.go should also match *.go in current directory
		baseMatches, err := filepath.Glob(suffix)
		if err == nil {
			matches = append(matches, baseMatches...)
		}
	}

	// Walk the directory tree
	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and vendor
		if d.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the base directory files if we already added them above
		if filepath.Dir(path) == basePath && (parts[0] == "" || parts[0] == "/") {
			return nil
		}

		// Check if file matches the suffix pattern
		if suffix == "" || matchSuffix(path, suffix) {
			matches = append(matches, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

// matchSuffix checks if a path matches a suffix pattern (e.g., "*.go")
func matchSuffix(path, pattern string) bool {
	if pattern == "" {
		return true
	}

	// Simple case: just extension matching
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:] // Remove *
		return strings.HasSuffix(path, ext)
	}

	// For more complex patterns, use filepath.Match on the basename
	base := filepath.Base(path)
	match, _ := filepath.Match(pattern, base)
	return match
}
