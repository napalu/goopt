package translations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// toGoName converts a string to a valid Go identifier
func toGoName(s string) string {
	// Handle special cases for format string patterns
	// These appear to be auto-generated keys from format strings
	if strings.Contains(s, "___") || strings.Contains(s, "__") {
		// For these special cases, preserve the distinction by encoding underscore counts
		s = strings.ReplaceAll(s, "___", "_Triple_")
		s = strings.ReplaceAll(s, "__", "_Double_")
	}

	// Replace common separators with underscores
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, " ", "_")

	// Split on underscores and capitalize each part
	parts := strings.Split(s, "_")
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, strings.ToUpper(part[:1])+part[1:])
		}
	}

	return strings.Join(result, "")
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

// TranslationUpdateMode defines how to handle existing keys
type TranslationUpdateMode string

const (
	UpdateModeSkip    TranslationUpdateMode = "skip"
	UpdateModeReplace TranslationUpdateMode = "replace"
	UpdateModeError   TranslationUpdateMode = "error"
)

// TranslationUpdateOptions contains options for updating translation files
type TranslationUpdateOptions struct {
	Mode       TranslationUpdateMode
	DryRun     bool
	Verbose    bool
	TodoPrefix string // Prefix for non-English translations (e.g., "[TODO]")
}

// TranslationUpdateResult contains the results of updating a translation file
type TranslationUpdateResult struct {
	Added    int
	Skipped  int
	Replaced int
	Modified bool // Whether the file was actually modified
}

// UpdateTranslationFile updates a translation file with new key-value pairs
func UpdateTranslationFile(filename string, keysToAdd map[string]string, opts TranslationUpdateOptions) (*TranslationUpdateResult, error) {
	result := &TranslationUpdateResult{}
	
	// Read existing content
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(filename); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	}
	
	// Detect language from filename
	isEnglish := strings.Contains(strings.ToLower(filename), "en.json") ||
		strings.Contains(strings.ToLower(filename), "english")
	
	// Process each key
	for key, value := range keysToAdd {
		if _, exists := existing[key]; exists {
			switch opts.Mode {
			case UpdateModeError:
				return nil, fmt.Errorf("key '%s' already exists in %s", key, filename)
			case UpdateModeSkip:
				result.Skipped++
			case UpdateModeReplace:
				// Apply TODO prefix for non-English files if provided
				if !isEnglish && opts.TodoPrefix != "" {
					value = fmt.Sprintf("%s %s", opts.TodoPrefix, value)
				}
				existing[key] = value
				result.Replaced++
				result.Modified = true
			}
		} else {
			// Apply TODO prefix for non-English files if provided
			if !isEnglish && opts.TodoPrefix != "" {
				value = fmt.Sprintf("%s %s", opts.TodoPrefix, value)
			}
			existing[key] = value
			result.Added++
			result.Modified = true
		}
	}
	
	// Write back if modified and not dry-run
	if result.Modified && !opts.DryRun {
		data, err := json.MarshalIndent(existing, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %w", err)
		}
		
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
	}
	
	return result, nil
}
