package translations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGlobalDeduplicator(t *testing.T) {
	// Create a test deduplicator
	dedup := NewGlobalDeduplicator()

	// Test adding strings
	tests := []struct {
		value      string
		key        string
		expectDupe bool
	}{
		{
			value:      "Hello World",
			key:        "app.greeting",
			expectDupe: false,
		},
		{
			value:      "Hello World", // Same value, different key
			key:        "app.welcome",
			expectDupe: true, // Should be detected as duplicate
		},
		{
			value:      "hello world", // Different case
			key:        "app.lowercase",
			expectDupe: false, // Case sensitive
		},
		{
			value:      "Hello %s",
			key:        "app.format_greeting",
			expectDupe: false,
		},
		{
			value:      "Hello %s", // Same format string
			key:        "app.another_format",
			expectDupe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			// Check if it's a duplicate
			existingResult, isDupe := dedup.CheckDuplicate(tt.value)

			if isDupe != tt.expectDupe {
				t.Errorf("CheckDuplicate(%q) = %v, want %v", tt.value, isDupe, tt.expectDupe)
			}

			if !isDupe {
				// Add it manually to the internal map (simulating what would happen during load)
				normalized := dedup.normalizeValue(tt.value)
				dedup.valueToKey[normalized] = &DeduplicationResult{
					ExistingKey:   tt.key,
					ExistingValue: tt.value,
					ExistingFile:  "test.json",
				}
				dedup.keyToValue[tt.key] = tt.value
			} else if existingResult != nil && existingResult.ExistingKey == tt.key {
				t.Errorf("Duplicate detected but returned same key: %q", tt.key)
			}
		})
	}
}

func TestGlobalDeduplicatorLoadFromFiles(t *testing.T) {
	// Create temp directory with test locale files
	tmpDir := t.TempDir()

	// Create test locale files with flat structure
	enFile := filepath.Join(tmpDir, "en.json")
	enContent := `{
		"app.existing.user_logged_in": "User logged in successfully",
		"app.extracted.hello_world": "Hello World",
		"app.extracted.error_404__not_found": "Error 404: Not found",
		"app.extracted.n0000_00_00_00_00_00": "0000_00_00_00_00_00"
	}`
	if err := os.WriteFile(enFile, []byte(enContent), 0644); err != nil {
		t.Fatalf("Failed to write en.json: %v", err)
	}

	deFile := filepath.Join(tmpDir, "de.json")
	deContent := `{
		"app.existing.user_logged_in": "[TODO] User logged in successfully",
		"app.extracted.hello_world": "Hallo Welt",
		"app.extracted.error_404__not_found": "[TODO] Error 404: Not found",
		"app.extracted.n0000_00_00_00_00_00": "[TODO] 0000_00_00_00_00_00"
	}`
	if err := os.WriteFile(deFile, []byte(deContent), 0644); err != nil {
		t.Fatalf("Failed to write de.json: %v", err)
	}

	// Test loading
	dedup := NewGlobalDeduplicator()
	if err := dedup.LoadFromFiles([]string{tmpDir + "/*.json"}); err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	// Test that existing strings are detected as duplicates
	testCases := []struct {
		value       string
		expectedKey string
		shouldExist bool
	}{
		{
			value:       "User logged in successfully",
			expectedKey: "app.existing.user_logged_in",
			shouldExist: true,
		},
		{
			value:       "Hello World",
			expectedKey: "app.extracted.hello_world",
			shouldExist: true,
		},
		{
			value:       "Error 404: Not found",
			expectedKey: "app.extracted.error_404__not_found",
			shouldExist: true,
		},
		{
			value:       "0000_00_00_00_00_00",
			expectedKey: "app.extracted.n0000_00_00_00_00_00",
			shouldExist: true,
		},
		{
			value:       "New string not in files",
			expectedKey: "",
			shouldExist: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			existingKey, exists := dedup.GetExistingKey(tc.value)

			if exists != tc.shouldExist {
				t.Errorf("GetExistingKey(%q) exists = %v, want %v", tc.value, exists, tc.shouldExist)
			}

			if exists && existingKey != tc.expectedKey {
				t.Errorf("GetExistingKey(%q) = %q, want %q", tc.value, existingKey, tc.expectedKey)
			}
		})
	}
}

func TestDeduplicatorValueNormalization(t *testing.T) {
	dedup := NewGlobalDeduplicator()

	// Test that format strings with same pattern but different whitespace are considered duplicates
	normalized := dedup.normalizeValue("Hello    World")
	dedup.valueToKey[normalized] = &DeduplicationResult{
		ExistingKey:   "app.multi_space",
		ExistingValue: "Hello    World",
		ExistingFile:  "test.json",
	}

	// This should NOT be a duplicate (normalization preserves exact whitespace)
	_, isDupe := dedup.CheckDuplicate("Hello World")
	if isDupe {
		t.Error("Different whitespace should not be considered duplicate")
	}

	// Test format string normalization
	normalized2 := dedup.normalizeValue("Error: %s occurred")
	dedup.valueToKey[normalized2] = &DeduplicationResult{
		ExistingKey:   "app.error_format",
		ExistingValue: "Error: %s occurred",
		ExistingFile:  "test.json",
	}

	// Same format string should be duplicate
	result, isDupe := dedup.CheckDuplicate("Error: %s occurred")
	if !isDupe {
		t.Error("Same format string should be detected as duplicate")
	}
	if result != nil && result.ExistingKey != "app.error_format" {
		t.Errorf("Expected key app.error_format, got %s", result.ExistingKey)
	}
}

func TestDeduplicatorWithNumericStrings(t *testing.T) {
	dedup := NewGlobalDeduplicator()

	// Add numeric strings
	numericStrings := []struct {
		value string
		key   string
	}{
		{"2023-12-25 00:00:00", "app.extracted.n2023_12_25_00_00_00"},
		{"0000_00_00_00_00_00", "app.extracted.n0000_00_00_00_00_00"},
		{"12345", "app.extracted.n12345"},
		{"123 items found", "app.extracted.n123_items_found"},
		{"Order #12345 processed", "app.extracted.order__12345_processed"},
	}

	// Add all strings
	for _, ns := range numericStrings {
		normalized := dedup.normalizeValue(ns.value)
		dedup.valueToKey[normalized] = &DeduplicationResult{
			ExistingKey:   ns.key,
			ExistingValue: ns.value,
			ExistingFile:  "test.json",
		}
		dedup.keyToValue[ns.key] = ns.value
	}

	// Verify they're all detected as duplicates when checked again
	for _, ns := range numericStrings {
		t.Run(ns.value, func(t *testing.T) {
			result, isDupe := dedup.CheckDuplicate(ns.value)
			if !isDupe {
				t.Errorf("String %q should be detected as duplicate", ns.value)
			}
			if result != nil && result.ExistingKey != ns.key {
				t.Errorf("Expected key %q, got %q", ns.key, result.ExistingKey)
			}
		})
	}
}
