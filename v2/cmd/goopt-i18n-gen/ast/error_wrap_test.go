package ast

import (
	"strings"
	"testing"
)

// TestErrorWrappingHandling tests that error wrapping with %w is handled correctly
func TestErrorWrappingHandling(t *testing.T) {
	// String map for testing
	stringMap := map[string]string{
		`"connection failed: %w"`:      "app.extracted.connection_failed_w",
		`"failed to read file %s: %w"`: "app.extracted.failed_to_read_file_s_w",
		`"database error"`:             "app.extracted.database_error",
		`"invalid input: %v"`:          "app.extracted.invalid_input_v",
		`"operation failed on %s: %w"`: "app.extracted.operation_failed_on_s_w",
		`"multiple errors: %w, %w"`:    "app.extracted.multiple_errors_w_w",
		`"failed to read file %s"`:     "app.extracted.failed_to_read_file_s",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple error wrapping",
			input:    `fmt.Errorf("connection failed: %w", err)`,
			expected: `fmt.Errorf("%s: %w", tr.T(messages.Keys.App.Extracted.ConnectionFailedW), err)`,
		},
		{
			name:     "error wrap with other format specifier",
			input:    `fmt.Errorf("failed to read file %s: %w", filename, err)`,
			expected: `fmt.Errorf("%s: %w", tr.T(messages.Keys.App.Extracted.FailedToReadFileSW, filename), err)`,
		},
		{
			name:     "errorf without wrap converts to errors.New",
			input:    `fmt.Errorf("database error")`,
			expected: `errors.New(tr.T(messages.Keys.App.Extracted.DatabaseError))`,
		},
		{
			name:     "errorf with %v but no %w",
			input:    `fmt.Errorf("invalid input: %v", input)`,
			expected: `errors.New(tr.T(messages.Keys.App.Extracted.InvalidInputV, input))`,
		},
		{
			name:     "complex format with wrap at end",
			input:    `fmt.Errorf("operation failed on %s: %w", server, originalErr)`,
			expected: `fmt.Errorf("%s: %w", tr.T(messages.Keys.App.Extracted.OperationFailedOnSW, server), originalErr)`,
		},
		{
			name:     "errors.Wrapf from errors package",
			input:    `errors.Wrapf(err, "failed to read file %s", filename)`,
			expected: `errors.Wrapf(err, "%s", tr.T(messages.Keys.App.Extracted.FailedToReadFileS, filename))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create full code
			code := `package main
import (
	"fmt"
	"errors"
)

func test() error {
	` + tt.input + `
}`

			transformer := NewFormatTransformer(stringMap)
			result, err := transformer.TransformFile("test.go", []byte(code))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			resultStr := string(result)
			if !strings.Contains(resultStr, tt.expected) {
				t.Errorf("Expected result to contain:\n%s\n\nGot:\n%s", tt.expected, resultStr)
			}

			// Additional checks for error wrapping
			if strings.Contains(tt.input, "%w") {
				// Should preserve fmt.Errorf for wrapping
				if !strings.Contains(resultStr, "fmt.Errorf") {
					t.Error("Error wrapping should preserve fmt.Errorf")
				}
				// Should have the pattern %s: %w
				if !strings.Contains(resultStr, `"%s: %w"`) {
					t.Error("Error wrapping should use the correct format pattern")
				}
			} else if strings.Contains(tt.input, "fmt.Errorf") {
				// Should convert to errors.New if no wrapping
				if !strings.Contains(resultStr, "errors.New") {
					t.Error("fmt.Errorf without error wrapping should convert to errors.New")
				}
			}
		})
	}
}

// TestErrorWrappingEdgeCases tests edge cases for error wrapping
func TestErrorWrappingEdgeCases(t *testing.T) {
	stringMap := map[string]string{
		`"error: %w at position %d"`: "app.extracted.error_w_at_position_d",
		`"wrap1: %w"`:                "app.extracted.wrap1_w",
		`"wrap2: %w"`:                "app.extracted.wrap2_w",
	}

	tests := []struct {
		name        string
		input       string
		shouldError bool
		contains    []string
		notContains []string
	}{
		{
			name:  "error wrap not at end",
			input: `fmt.Errorf("error: %w at position %d", err, pos)`,
			contains: []string{
				"fmt.Errorf",
				`"%s: %w"`,
				"tr.T(messages.Keys.App.Extracted.ErrorWAtPositionD",
			},
			notContains: []string{
				"errors.New",
			},
		},
		{
			name:  "multiple error wraps in format string",
			input: `fmt.Errorf("multiple errors: %w, %w", err1, err2)`,
			contains: []string{
				"fmt.Errorf",
				// Should still transform even with multiple %w
			},
		},
		{
			name:  "nested Errorf calls",
			input: `fmt.Errorf("wrap1: %w", fmt.Errorf("wrap2: %w", baseErr))`,
			contains: []string{
				"fmt.Errorf",
				"tr.T(messages.Keys.App.Extracted.Wrap1W)",
				"tr.T(messages.Keys.App.Extracted.Wrap2W)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := `package main
import "fmt"

func test() error {
	` + tt.input + `
}`

			transformer := NewFormatTransformer(stringMap)
			result, err := transformer.TransformFile("test.go", []byte(code))
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			resultStr := string(result)

			for _, expected := range tt.contains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected result to contain: %s", expected)
				}
			}

			for _, unexpected := range tt.notContains {
				if strings.Contains(resultStr, unexpected) {
					t.Errorf("Result should not contain: %s", unexpected)
				}
			}
		})
	}
}
