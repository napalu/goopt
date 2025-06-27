package translations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
)

// toGoName converts a string to a valid Go identifier
func toGoName(s string) string {
	// Split by dots first to preserve hierarchy
	dotParts := strings.Split(s, ".")
	var result []string

	for _, part := range dotParts {
		// Replace common separators with underscores
		part = strings.ReplaceAll(part, "-", "_")
		part = strings.ReplaceAll(part, " ", "_")

		// Split on underscores and capitalize each part
		subParts := strings.Split(part, "_")
		var pascalPart []string
		for _, subPart := range subParts {
			if subPart != "" {
				pascalPart = append(pascalPart, strings.ToUpper(subPart[:1])+subPart[1:])
			}
		}

		if len(pascalPart) > 0 {
			result = append(result, strings.Join(pascalPart, ""))
		}
	}

	return strings.Join(result, ".")
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
		return fmt.Errorf(messages.Keys.AppError.FailedToCreateDir, dir, err)
	}

	// Create empty JSON file
	emptyJSON := []byte("{}")
	if err := os.WriteFile(path, emptyJSON, 0644); err != nil {
		return fmt.Errorf(messages.Keys.AppError.FailedToCreateFile, path, err)
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
			return nil, fmt.Errorf(messages.Keys.AppError.FailedToExpandPattern, pattern, err)
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
		return nil, fmt.Errorf(messages.Keys.AppError.NoFiles)
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
			return nil, fmt.Errorf(messages.Keys.AppError.FailedToParseJsonSimple, err)
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
				return nil, fmt.Errorf(messages.Keys.AppError.KeyAlreadyExists, key, filename)
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
			return nil, fmt.Errorf(messages.Keys.AppError.FailedToMarshalJson, err)
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			return nil, fmt.Errorf(messages.Keys.AppError.FailedToWriteFile, err)
		}
	}

	return result, nil
}
