package translations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// toGoName converts a string to a valid Go identifier
func toGoName(s string) string {
	// Replace common separators with underscores
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, " ", "_")

	// Split on underscores and capitalize each part
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}

	return strings.Join(parts, "")
}

// ensureInputFile creates the directory and file if they don't exist
func ensureInputFile(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		return nil // File exists, nothing to do
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create empty JSON file
	emptyJSON := []byte("{}")
	if err := os.WriteFile(path, emptyJSON, 0644); err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}

	fmt.Printf("✓ Created %s\n", path)
	return nil
}

// expandInputFiles expands wildcards in input paths and returns all matching files
func expandInputFiles(inputs []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	for _, pattern := range inputs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to expand pattern %s: %w", pattern, err)
		}

		// If no matches, treat as literal file
		if len(matches) == 0 {
			matches = []string{pattern}
		}

		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				files = append(files, match)
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no input files found")
	}

	return files, nil
}
