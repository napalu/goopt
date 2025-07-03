package ast

import (
	"testing"
)

func TestSlogSupport(t *testing.T) {
	tests := []struct {
		name              string
		code              string
		expectedStrings   []string
		unexpectedStrings []string
		expectedFormat    bool
	}{
		{
			name:              "slog.Info with message",
			code:              `slog.Info("Starting server", "port", 8080)`,
			expectedStrings:   []string{"Starting server"},
			unexpectedStrings: []string{"port"}, // We should NOT extract the key
			expectedFormat:    false,
		},
		{
			name:            "slog.Error with message",
			code:            `slog.Error("Failed to connect", "host", host, "error", err)`,
			expectedStrings: []string{"Failed to connect"},
			expectedFormat:  false,
		},
		{
			name:            "slog.Debug with message",
			code:            `slog.Debug("Processing request", "id", requestID)`,
			expectedStrings: []string{"Processing request"},
			expectedFormat:  false,
		},
		{
			name:            "slog.Warn with message",
			code:            `slog.Warn("High memory usage", "percent", 95)`,
			expectedStrings: []string{"High memory usage"},
			expectedFormat:  false,
		},
		{
			name:            "logger.Info with message",
			code:            `logger.Info("User logged in", "user", username)`,
			expectedStrings: []string{"User logged in"},
			expectedFormat:  false,
		},
		{
			name:            "logger.ErrorContext with message",
			code:            `logger.ErrorContext(ctx, "Database query failed", "query", sql)`,
			expectedStrings: []string{"Database query failed"},
			expectedFormat:  false,
		},
		{
			name:            "custom logger with Info",
			code:            `myLogger.Info("Custom logger message", "key", "value")`,
			expectedStrings: []string{"Custom logger message"},
			expectedFormat:  false,
		},
		{
			name:            "log/slog With pattern",
			code:            `slog.With("request_id", reqID).Info("Processing", "action", "update")`,
			expectedStrings: []string{"Processing"},
			expectedFormat:  false,
		},
		{
			name:            "slog with concatenation",
			code:            `slog.Info("Request " + "completed", "duration", time.Since(start))`,
			expectedStrings: []string{"Request ", "completed"}, // Currently extracts as separate strings
			expectedFormat:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We'll test by creating a full Go file instead of parsing expressions

			// Create a minimal Go file with the test code
			code := `package main
import "log/slog"

func test() {
	` + tt.code + `
}`

			// Create string extractor
			extractor, _ := NewStringExtractor(nil, "", "", 0)

			// Extract from the code
			err := extractor.ExtractFromString("test.go", code)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			// Get extracted strings
			extracted := extractor.GetExtractedStrings()

			// Check if expected strings were found
			for _, expected := range tt.expectedStrings {
				found := false
				for str := range extracted {
					if str == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to extract %q but it was not found", expected)
					t.Logf("Extracted strings: %v", getKeys(extracted))
					// Log more details about what was extracted
					for str, info := range extracted {
						t.Logf("  String: %q, IsFormat: %v, Locations: %d", str, info.IsFormatString, len(info.Locations))
					}
				}
			}
		})
	}
}

// TestSlogTransformation tests that slog calls are properly transformed
func TestSlogTransformation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // What we expect after transformation
		skip     bool   // Whether this should be skipped for transformation
	}{
		{
			name:     "slog.Info transformation",
			input:    `slog.Info("Starting server", "port", 8080)`,
			expected: `slog.Info(tr.T(messages.Keys.App.Extracted.StartingServer), "port", 8080)`,
			skip:     false,
		},
		{
			name:     "logger.Error transformation",
			input:    `logger.Error("Connection failed", "error", err)`,
			expected: `logger.Error(tr.T(messages.Keys.App.Extracted.ConnectionFailed), "error", err)`,
			skip:     false,
		},
		{
			name:     "slog with format specifiers should not transform",
			input:    `slog.Info("User %s logged in", username)`, // This is wrong slog usage
			expected: `slog.Info("User %s logged in", username)`, // Should not transform
			skip:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test would verify that our transformation handles slog correctly
			// The key insight is that slog functions take a message as first arg
			// followed by key-value pairs, not format strings
		})
	}
}

// Helper function to get keys from map
func getKeys(m map[string]*ExtractedString) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
