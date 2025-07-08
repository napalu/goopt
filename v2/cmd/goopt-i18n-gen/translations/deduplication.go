package translations

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// DeduplicationResult contains information about deduplicated strings
type DeduplicationResult struct {
	ExistingKey   string
	ExistingValue string
	ExistingFile  string
}

// GlobalDeduplicator handles deduplication across all locale files
type GlobalDeduplicator struct {
	// Map of string value to its existing key and file
	valueToKey map[string]*DeduplicationResult
	// Map of key to its value for quick lookup
	keyToValue map[string]string
}

// NewGlobalDeduplicator creates a new deduplicator
func NewGlobalDeduplicator() *GlobalDeduplicator {
	return &GlobalDeduplicator{
		valueToKey: make(map[string]*DeduplicationResult),
		keyToValue: make(map[string]string),
	}
}

// LoadFromFiles loads existing translations from locale files
func (gd *GlobalDeduplicator) LoadFromFiles(patterns []string) error {
	files, err := expandInputFiles(patterns)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := gd.loadFile(file); err != nil {
			// Continue loading other files even if one fails
			fmt.Printf("Warning: failed to load %s: %v\n", file, err)
			continue
		}
	}

	return nil
}

// loadFile loads translations from a single file
func (gd *GlobalDeduplicator) loadFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's OK
		}
		return err
	}

	if len(data) == 0 {
		return nil // Empty file
	}

	// Parse JSON - expecting flat structure
	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return err
	}

	// Process all key-value pairs (flat structure)
	for key, value := range translations {
		// Clean the key - remove any "messages.Keys." prefix that shouldn't be in locale files
		cleanKey := key
		if strings.HasPrefix(cleanKey, "messages.Keys.") {
			cleanKey = strings.TrimPrefix(cleanKey, "messages.Keys.")
			// Convert from Go naming back to key format: App.Extracted.HelloWorld -> app.extracted.hello_world
			cleanKey = gd.convertFromGoFormat(cleanKey)
		}

		// Store the mapping with clean key
		gd.keyToValue[cleanKey] = value

		// For deduplication, we normalize the string
		normalizedValue := gd.normalizeValue(value)

		// Only store the first occurrence of each value
		if _, exists := gd.valueToKey[normalizedValue]; !exists {
			gd.valueToKey[normalizedValue] = &DeduplicationResult{
				ExistingKey:   cleanKey,
				ExistingValue: value,
				ExistingFile:  filename,
			}
		}
	}

	return nil
}

// normalizeValue normalizes a string value for deduplication comparison
func (gd *GlobalDeduplicator) normalizeValue(value string) string {
	// Trim whitespace
	normalized := strings.TrimSpace(value)

	// Optionally: lowercase for case-insensitive comparison
	// normalized = strings.ToLower(normalized)

	// Remove TODO prefixes if any
	prefixes := []string{"[TODO]", "[TRANSLATE]", "TODO:", "FIXME:"}
	for _, prefix := range prefixes {
		normalized = strings.TrimSpace(strings.TrimPrefix(normalized, prefix))
	}

	return normalized
}

// convertFromGoFormat converts Go naming back to key format
// e.g. "App.Extracted.HelloWorld" -> "app.extracted.hello_world"
func (gd *GlobalDeduplicator) convertFromGoFormat(goFormat string) string {
	parts := strings.Split(goFormat, ".")
	var keyParts []string

	for _, part := range parts {
		// Convert from PascalCase to snake_case
		snakeCase := ""
		for i, r := range part {
			if i > 0 && r >= 'A' && r <= 'Z' {
				snakeCase += "_"
			}
			snakeCase += strings.ToLower(string(r))
		}
		keyParts = append(keyParts, snakeCase)
	}

	return strings.Join(keyParts, ".")
}

// CheckDuplicate checks if a value already exists
func (gd *GlobalDeduplicator) CheckDuplicate(value string) (*DeduplicationResult, bool) {
	normalized := gd.normalizeValue(value)
	if result, exists := gd.valueToKey[normalized]; exists {
		return result, true
	}
	return nil, false
}

// GetExistingKey returns the existing key for a value if it exists
func (gd *GlobalDeduplicator) GetExistingKey(value string) (string, bool) {
	if result, exists := gd.CheckDuplicate(value); exists {
		return result.ExistingKey, true
	}
	return "", false
}

// GetAllKeys returns all known keys
func (gd *GlobalDeduplicator) GetAllKeys() []string {
	keys := make([]string, 0, len(gd.keyToValue))
	for key := range gd.keyToValue {
		keys = append(keys, key)
	}
	return keys
}

// GenerateUniqueKey generates a unique key based on the base key
func (gd *GlobalDeduplicator) GenerateUniqueKey(baseKey string) string {
	if _, exists := gd.keyToValue[baseKey]; !exists {
		return baseKey
	}

	// Try with numeric suffixes
	for i := 2; i < 1000; i++ {
		candidateKey := fmt.Sprintf("%s_%d", baseKey, i)
		if _, exists := gd.keyToValue[candidateKey]; !exists {
			return candidateKey
		}
	}

	// Fallback to timestamp-based suffix
	return fmt.Sprintf("%s_%d", baseKey, os.Getpid())
}
