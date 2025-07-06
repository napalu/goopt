package util

import (
	"strings"
	"testing"
)

func TestKeyToGoName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{
			name:     "simple snake case",
			input:    "hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "single word",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},

		// Numeric cases - critical for our fix
		{
			name:     "all numeric with underscores",
			input:    "0000_00_00_00_00_00",
			expected: "N00000000000000",
		},
		{
			name:     "date format numeric",
			input:    "2023_12_25_00_00_00",
			expected: "N20231225000000",
		},
		{
			name:     "simple numeric",
			input:    "12345",
			expected: "N12345",
		},
		{
			name:     "numeric at start",
			input:    "404_not_found",
			expected: "N404NotFound",
		},
		{
			name:     "numeric in middle",
			input:    "error_404_not_found",
			expected: "ErrorN404NotFound",
		},
		{
			name:     "text with numbers",
			input:    "order_12345_processed",
			expected: "OrderN12345Processed",
		},
		{
			name:     "leading number with text",
			input:    "123_items_found",
			expected: "N123ItemsFound",
		},

		// Special characters (double underscore)
		{
			name:     "double underscore for special chars",
			input:    "error_404__not_found",
			expected: "ErrorN404NotFound",
		},
		{
			name:     "format string markers",
			input:    "removed_child_group__s_from_group__s",
			expected: "RemovedChildGroupSFromGroupS",
		},
		{
			name:     "multiple double underscores",
			input:    "removed_child_group__s_from_group__s2",
			expected: "RemovedChildGroupSFromGroupS2",
		},

		// Edge cases
		{
			name:     "only underscores",
			input:    "___",
			expected: "",
		},
		{
			name:     "mixed case preservation",
			input:    "HTTP_response",
			expected: "HTTPResponse",
		},
		{
			name:     "acronym handling",
			input:    "api_key",
			expected: "ApiKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KeyToGoName(tt.input)
			if result != tt.expected {
				t.Errorf("KeyToGoName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateKeyFromString(t *testing.T) {
	prefix := "app.extracted"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{
			name:     "simple string",
			input:    "Hello World",
			expected: "app.extracted.hello_world",
		},
		{
			name:     "with punctuation",
			input:    "Hello, World!",
			expected: "app.extracted.hello__world",
		},
		{
			name:     "with numbers",
			input:    "Error 404: Not found",
			expected: "app.extracted.error_404__not_found",
		},

		// Numeric strings
		{
			name:     "date time string",
			input:    "2023-12-25 00:00:00",
			expected: "app.extracted.n2023_12_25_00_00_00",
		},
		{
			name:     "all numeric with special chars",
			input:    "0000_00_00_00_00_00",
			expected: "app.extracted.n0000_00_00_00_00_00",
		},
		{
			name:     "pure numeric",
			input:    "12345",
			expected: "app.extracted.n12345",
		},
		{
			name:     "order number",
			input:    "Order #12345 processed",
			expected: "app.extracted.order__12345_processed",
		},
		{
			name:     "number items",
			input:    "123 items found",
			expected: "app.extracted.n123_items_found",
		},

		// Format strings
		{
			name:     "printf format",
			input:    "Hello %s",
			expected: "app.extracted.hello__s",
		},
		{
			name:     "multiple format specifiers",
			input:    "Removed child group %s from group %s",
			expected: "app.extracted.removed_child_group__s_from_group__s2",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "app.extracted.",
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: "app.extracted.",
		},
		{
			name:     "unicode characters",
			input:    "Hello 世界",
			expected: "app.extracted.hello",
		},
		{
			name:     "mixed case",
			input:    "HTTPResponse",
			expected: "app.extracted.httpresponse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateKeyFromString(prefix, tt.input)
			if result != tt.expected {
				t.Errorf("GenerateKeyFromString(%q, %q) = %q, want %q", prefix, tt.input, result, tt.expected)
			}
		})
	}
}

// TestKeyToGoNameAndGenerateKeyFromStringConsistency ensures that the key generation
// and Go name conversion are consistent with each other
func TestKeyToGoNameAndGenerateKeyFromStringConsistency(t *testing.T) {
	prefix := "app.extracted"
	testStrings := []string{
		"Error 404: Not found",
		"2023-12-25 00:00:00",
		"0000_00_00_00_00_00",
		"123 items found",
		"12345",
		"Order #12345 processed",
		"Removed child group %s from group %s",
		"User logged in successfully",
	}

	for _, str := range testStrings {
		t.Run(str, func(t *testing.T) {
			// Generate key from string
			key := GenerateKeyFromString(prefix, str)

			// Extract just the last part (after prefix)
			parts := strings.Split(key, ".")
			if len(parts) < 3 {
				t.Fatalf("Expected key with at least 3 parts, got %q", key)
			}
			lastPart := parts[len(parts)-1]

			// Convert key part to Go name
			goName := KeyToGoName(lastPart)

			// The Go name should be a valid identifier
			if len(goName) > 0 {
				firstChar := rune(goName[0])
				if !((firstChar >= 'A' && firstChar <= 'Z') || (firstChar >= 'a' && firstChar <= 'z') || firstChar == '_') {
					t.Errorf("KeyToGoName produced invalid Go identifier: %q (from key %q, original string %q)", goName, lastPart, str)
				}
			}

			// Log the transformation for debugging
			t.Logf("String %q -> Key %q -> GoName %q", str, key, goName)
		})
	}
}
