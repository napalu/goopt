package ast

import (
	"regexp"
	"testing"
)

func TestSlogExclusionRegex(t *testing.T) {
	// Test different regex patterns to exclude slog keys
	testCases := []struct {
		name           string
		regex          string
		shouldMatch    []string // These should be excluded
		shouldNotMatch []string // These should be kept
	}{
		{
			name:           "Single word lowercase keys",
			regex:          `^[a-z]+$`,
			shouldMatch:    []string{"port", "host", "error", "query", "duration"},
			shouldNotMatch: []string{"Starting server", "Database error", "User logged in"},
		},
		{
			name:           "Snake_case keys",
			regex:          `^[a-z]+(_[a-z]+)*$`,
			shouldMatch:    []string{"port", "host", "error", "request_id", "user_name", "error_code"},
			shouldNotMatch: []string{"Starting server", "Database error", "User logged in"},
		},
		{
			name:           "Common log field names",
			regex:          `^(error|err|host|port|user|id|status|code|duration|latency|method|path|query|request_id|user_id|user_name|timestamp|level|msg|message|logger|source|file|line|function|stack|trace)$`,
			shouldMatch:    []string{"error", "host", "port", "duration", "user_id", "request_id"},
			shouldNotMatch: []string{"Starting server", "Database error", "Failed to connect to host"},
		},
		{
			name:           "Camelcase and lowercase identifiers",
			regex:          `^[a-z][a-zA-Z0-9]*$`,
			shouldMatch:    []string{"port", "hostName", "userId", "requestID", "errorCode"},
			shouldNotMatch: []string{"Starting server", "Database error", "User logged in", "ID"},
		},
		{
			name:           "Short strings (likely to be keys)",
			regex:          `^.{1,15}$`,
			shouldMatch:    []string{"port", "host", "error", "userId"},
			shouldNotMatch: []string{"Starting server with configuration", "Database connection failed"},
		},
		{
			name:           "No spaces (keys don't have spaces)",
			regex:          `^[^\s]+$`,
			shouldMatch:    []string{"port", "host", "error", "user_id", "request-id"},
			shouldNotMatch: []string{"Starting server", "Database error", "User logged in"},
		},
		{
			name:           "Combined: no spaces AND common patterns",
			regex:          `^([a-z][a-zA-Z0-9_]*|[a-z]+(_[a-z]+)*)$`,
			shouldMatch:    []string{"port", "hostName", "user_id", "requestID"},
			shouldNotMatch: []string{"Starting server", "Database error", "User logged in"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			re, err := regexp.Compile(tc.regex)
			if err != nil {
				t.Fatalf("Failed to compile regex: %v", err)
			}

			// Test strings that should match (be excluded)
			for _, str := range tc.shouldMatch {
				if !re.MatchString(str) {
					t.Errorf("Regex should match (exclude) %q but didn't", str)
				}
			}

			// Test strings that should NOT match (be kept)
			for _, str := range tc.shouldNotMatch {
				if re.MatchString(str) {
					t.Errorf("Regex should NOT match (keep) %q but did", str)
				}
			}
		})
	}
}

func TestSlogExclusionWithExtractor(t *testing.T) {
	code := `package main
import "log/slog"

func test() {
	slog.Info("Starting server", "port", 8080, "host", "localhost")
	slog.Error("Database connection failed", "error", err, "retry_count", 3)
	logger.Info("User logged in", "user_id", userID, "session_id", sessionID)
	slog.Debug("Processing request", "method", "GET", "path", "/api/users")
}`

	// Test with different skip patterns
	skipPatterns := []struct {
		name             string
		pattern          string
		expectedExcluded []string
		expectedKept     []string
	}{
		{
			name:             "Skip single word lowercase",
			pattern:          `^[a-z]+(_[a-z]+)*$`,
			expectedExcluded: []string{"port", "host", "error", "method", "path", "user_id", "session_id", "retry_count"},
			expectedKept:     []string{"Starting server", "Database connection failed", "User logged in", "Processing request"},
		},
		{
			name:             "Skip no-space strings",
			pattern:          `^[^\s]+$`,
			expectedExcluded: []string{"port", "host", "error", "localhost", "/api/users", "GET"},
			expectedKept:     []string{"Starting server", "Database connection failed", "User logged in", "Processing request"},
		},
		{
			name:             "Skip short identifiers",
			pattern:          `^[a-z][a-zA-Z0-9_]{0,20}$`,
			expectedExcluded: []string{"port", "host", "error", "method", "path", "user_id", "session_id", "retry_count"},
			expectedKept:     []string{"Starting server", "Database connection failed", "User logged in", "Processing request"},
		},
	}

	for _, sp := range skipPatterns {
		t.Run(sp.name, func(t *testing.T) {
			// Create extractor with skip pattern
			extractor, err := NewStringExtractor(nil, "", sp.pattern, 0)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			// Extract strings
			err = extractor.ExtractFromString("test.go", code)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			extracted := extractor.GetExtractedStrings()

			// Check that excluded strings are not present
			for _, excluded := range sp.expectedExcluded {
				if _, found := extracted[excluded]; found {
					t.Errorf("Expected %q to be excluded by pattern %s", excluded, sp.pattern)
				}
			}

			// Check that kept strings are present
			for _, kept := range sp.expectedKept {
				if _, found := extracted[kept]; !found {
					t.Errorf("Expected %q to be kept with pattern %s", kept, sp.pattern)
				}
			}

			// Log what was extracted
			t.Logf("Pattern %s extracted %d strings:", sp.pattern, len(extracted))
			for str := range extracted {
				t.Logf("  - %q", str)
			}
		})
	}
}
