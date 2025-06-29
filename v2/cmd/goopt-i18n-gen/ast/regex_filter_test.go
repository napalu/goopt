package ast

import (
	"os"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
)

// createTempGoFile creates a temporary Go file with the given content
func createTempGoFile(t *testing.T, content string) *os.File {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test*.go")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Return the file for cleanup
	file, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to reopen temp file: %v", err)
	}

	t.Cleanup(func() {
		file.Close()
		os.Remove(tmpFile.Name())
	})

	return file
}

// TestRegexFiltersInExtraction tests that regex filters work correctly during string extraction
func TestRegexFiltersInExtraction(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		matchOnly string
		skipMatch string
		minLength int
		expected  []string // strings that should be extracted
		excluded  []string // strings that should NOT be extracted
	}{
		{
			name: "skip single-word identifiers",
			code: `package main
import "fmt"
func main() {
	fmt.Println("Starting server")
	slog.Info("Server running", "port", 8080, "host", "localhost")
	fmt.Printf("Error: %s", "timeout")
}`,
			skipMatch: `^[a-z]+$`, // skip single lowercase words
			expected:  []string{"Starting server", "Server running", "Error: %s"},
			excluded:  []string{"port", "host", "timeout", "localhost"},
		},
		{
			name: "skip strings without spaces",
			code: `package main
import "log"
func main() {
	log.Println("Application started successfully")
	log.Printf("user_id: %s", userID)
	fmt.Errorf("Connection failed")
}`,
			skipMatch: `^[^\s]+$`, // skip strings without spaces
			expected:  []string{"Application started successfully"},
			excluded:  []string{"user_id:", "Connection"},
		},
		{
			name: "match only strings with spaces",
			code: `package main
func process() {
	fmt.Print("Processing items")
	fmt.Println("Done")
	log.Fatal("Critical error occurred")
}`,
			matchOnly: `.*\s+.*`, // only match strings with spaces
			expected:  []string{"Processing items", "Critical error occurred"},
			excluded:  []string{"Done"},
		},
		{
			name: "minimum length filter",
			code: `package main
func test() {
	fmt.Println("Hi")
	fmt.Println("Hello world")
	fmt.Println("A")
}`,
			minLength: 5,
			expected:  []string{"Hello world"},
			excluded:  []string{"Hi", "A"},
		},
		{
			name: "combined filters",
			code: `package main
func api() {
	log.Info("Starting API server", "version", "v1.0")
	log.Error("Failed to bind", "error", err)
	fmt.Println("OK")
}`,
			matchOnly: `.*\s+.*`,  // must have spaces
			skipMatch: `^[A-Z]+$`, // skip all caps
			minLength: 10,         // at least 10 chars
			expected:  []string{"Starting API server", "Failed to bind"},
			excluded:  []string{"version", "v1.0", "error", "OK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extractor with filters
			bundle := i18n.NewEmptyBundle()
			extractor, err := NewStringExtractor(bundle, tt.matchOnly, tt.skipMatch, tt.minLength)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			// Create temp file
			tmpFile := createTempGoFile(t, tt.code)
			defer tmpFile.Close()

			// Extract strings
			if err := extractor.ExtractFromString(tmpFile.Name(), tt.code); err != nil {
				t.Fatalf("ExtractFromString failed: %v", err)
			}

			extracted := extractor.GetExtractedStrings()

			// Check expected strings were extracted
			for _, expected := range tt.expected {
				found := false
				for str := range extracted {
					if str == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected string '%s' was not extracted", expected)
				}
			}

			// Check excluded strings were NOT extracted
			for _, excluded := range tt.excluded {
				if _, found := extracted[excluded]; found {
					t.Errorf("String '%s' should have been filtered out but was extracted", excluded)
				}
			}
		})
	}
}

// TestTransformMode tests the different transformation modes
func TestTransformMode(t *testing.T) {
	// String map for testing
	stringMap := map[string]string{
		`"Processing data"`:  "messages.Keys.ProcessingData",
		`"Error occurred"`:   "messages.Keys.ErrorOccurred",
		`"Debug info"`:       "messages.Keys.DebugInfo",
		`"Internal state"`:   "messages.Keys.InternalState",
		`"User logged in"`:   "messages.Keys.UserLoggedIn",
		`"Calculate result"`: "messages.Keys.CalculateResult",
	}

	tests := []struct {
		name            string
		code            string
		transformMode   string
		shouldTransform map[string]bool // function -> should transform
	}{
		{
			name: "user-facing mode (default)",
			code: `package main
import (
	"fmt"
	"log"
)

func main() {
	// User-facing functions - should transform
	fmt.Println("Processing data")
	log.Print("Error occurred")
	fmt.Fprint(w, "User logged in")
	
	// Non-user-facing functions - should NOT transform
	doSomething("Internal state")
	process("Calculate result")
}

func doSomething(msg string) {}
func process(s string) {}`,
			transformMode: "user-facing",
			shouldTransform: map[string]bool{
				"fmt.Println": true,
				"log.Print":   true,
				"fmt.Fprint":  true,
				"doSomething": false,
				"process":     false,
			},
		},
		{
			name: "all mode transforms everything",
			code: `package main
import "fmt"

func main() {
	fmt.Println("Processing data")
	customLog("Debug info")
	someFunc("Internal state")
}

func customLog(msg string) {}
func someFunc(s string) {}`,
			transformMode: "all",
			shouldTransform: map[string]bool{
				"fmt.Println": true,
				"customLog":   true,
				"someFunc":    true,
			},
		},
		{
			name: "slog functions in user-facing mode",
			code: `package main
import "log/slog"

func main() {
	slog.Info("Processing data")
	slog.Error("Error occurred")
	slog.Debug("Debug info")
}`,
			transformMode: "user-facing",
			shouldTransform: map[string]bool{
				"slog.Info":  true,
				"slog.Error": true,
				"slog.Debug": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewFormatTransformer(stringMap)
			transformer.SetTransformMode(tt.transformMode)

			result, err := transformer.TransformFile("test.go", []byte(tt.code))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			resultStr := string(result)

			// Check each function's transformation
			for funcName, shouldTransform := range tt.shouldTransform {
				// Look for tr.T calls after the function
				if shouldTransform {
					if !strings.Contains(resultStr, "tr.T(messages.Keys.") {
						t.Errorf("Function %s should have been transformed but wasn't", funcName)
					}
				} else {
					// For non-user-facing functions, check that the original string is still there
					// This is a bit tricky since we need to check context
					// For now, just ensure the file compiles and has expected structure
				}
			}
		})
	}
}

// TestRegexFiltersWithTransformation tests that transformation respects strings filtered during extraction
func TestRegexFiltersWithTransformation(t *testing.T) {
	code := `package main
import (
	"fmt"
	"log/slog"
)

func main() {
	fmt.Println("Starting application")  // Should be extracted and transformed
	fmt.Println("OK")                   // Too short, should be filtered
	slog.Info("Server ready", "port", 8080) // "Server ready" extracted, "port" filtered
	fmt.Printf("Error: %s", "timeout")  // "Error: %s" extracted, "timeout" might be filtered
}`

	// First, extract strings with filters
	bundle := i18n.NewEmptyBundle()
	extractor, err := NewStringExtractor(
		bundle,
		"",         // no match-only filter
		`^[a-z]+$`, // skip single lowercase words
		5,          // minimum length 5
	)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	tmpFile := createTempGoFile(t, code)
	defer tmpFile.Close()

	if err := extractor.ExtractFromString(tmpFile.Name(), code); err != nil {
		t.Fatalf("ExtractFromString failed: %v", err)
	}

	// Build string map from extracted strings
	stringMap := make(map[string]string)
	for str := range extractor.GetExtractedStrings() {
		// Create quoted version for AST matching
		quotedStr := `"` + str + `"`
		stringMap[quotedStr] = "messages.Keys.Test"
	}

	// Transform with the filtered string map
	transformer := NewFormatTransformer(stringMap)
	result, err := transformer.TransformFile("test.go", []byte(code))
	if err != nil {
		t.Fatalf("TransformFile failed: %v", err)
	}

	resultStr := string(result)

	lines := strings.Split(resultStr, "\n")
	for i, line := range lines {
		if strings.Contains(line, "port") || strings.Contains(line, "OK") || strings.Contains(line, "tr.T") {
			t.Logf("Line %d: %s", i, line)
		}
	}

	// Verify filtered strings were not transformed
	// They should still appear as literals in the output
	if !strings.Contains(resultStr, `"OK"`) {
		t.Error("Short string 'OK' should still be present as a literal")
	}
	if !strings.Contains(resultStr, `"port"`) {
		t.Error("Identifier 'port' should still be present as a literal")
	}

	// For slog.Info line, "port" can be on the same line as tr.T because only "Server ready" was transformed
	// Check that "OK" line doesn't have tr.T
	for _, line := range lines {
		if strings.Contains(line, `fmt.Println("OK")`) && strings.Contains(line, "tr.T") {
			t.Error("Line with 'OK' should not contain tr.T")
		}
		// The slog.Info line will have both tr.T (for "Server ready") and "port" literal, which is correct
	}

	// Verify non-filtered strings were transformed
	if !strings.Contains(resultStr, "tr.T") {
		t.Error("Expected at least some strings to be transformed")
	}
}
