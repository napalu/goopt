package ast

import (
	"strings"
	"testing"
)

// TestErrorHandlingDecisionFlow tests the complete decision flow for error handling
func TestErrorHandlingDecisionFlow(t *testing.T) {
	stringMap := map[string]string{
		// Error scenarios
		`"database connection failed: %w"`:        "messages.Keys.DatabaseConnectionFailedW",
		`"validation failed"`:                     "messages.Keys.ValidationFailed", 
		`"timeout occurred: %v"`:                  "messages.Keys.TimeoutOccurredV",
		`"failed to process %s: %w"`:              "messages.Keys.FailedToProcessSW",
		`"context info: %s"`:                      "messages.Keys.ContextInfoS",
		
		// Non-error scenarios  
		`"user logged in: %s"`:                    "messages.Keys.UserLoggedInS",
		`"processing item %d"`:                    "messages.Keys.ProcessingItemD",
		`"result: %v"`:                            "messages.Keys.ResultV",
	}

	tests := []struct {
		name                string
		code                string
		expectErrorHandling bool
		expectErrorWrap     bool
		expectFuncChange    bool // Whether function name should change
		expectedOutput      []string // Patterns that should appear
		forbiddenOutput     []string // Patterns that should NOT appear
		description         string
	}{
		{
			name:                "fmt.Errorf with %w - preserve wrapping",
			code:                `fmt.Errorf("database connection failed: %w", dbErr)`,
			expectErrorHandling: true,
			expectErrorWrap:     true,
			expectFuncChange:    false, // Keep fmt.Errorf
			expectedOutput:      []string{`fmt.Errorf("%s: %w"`, "tr.T(messages.Keys.DatabaseConnectionFailedW)", "dbErr"},
			forbiddenOutput:     []string{"errors.New", "fmt.Print"},
			description:         "Error with %w should preserve fmt.Errorf and error wrapping",
		},
		{
			name:                "fmt.Errorf without %w - convert to errors.New",
			code:                `fmt.Errorf("validation failed")`,
			expectErrorHandling: true,
			expectErrorWrap:     false,
			expectFuncChange:    true, // Change to errors.New
			expectedOutput:      []string{"errors.New", "tr.T(messages.Keys.ValidationFailed)"},
			forbiddenOutput:     []string{"fmt.Errorf", "%w", "fmt.Print"},
			description:         "Error without %w should convert to errors.New",
		},
		{
			name:                "fmt.Errorf with format but no %w",
			code:                `fmt.Errorf("timeout occurred: %v", duration)`,
			expectErrorHandling: true,
			expectErrorWrap:     false,
			expectFuncChange:    true,
			expectedOutput:      []string{"errors.New", "tr.T(messages.Keys.TimeoutOccurredV", "duration"},
			forbiddenOutput:     []string{"fmt.Errorf", "%w"},
			description:         "Error with format specifiers but no %w should convert to errors.New",
		},
		{
			name:                "errors.Wrapf - preserve wrapping",
			code:                `errors.Wrapf(originalErr, "context info: %s", ctx)`,
			expectErrorHandling: true,
			expectErrorWrap:     false, // Wrapf doesn't use %w, it wraps by design
			expectFuncChange:    false, // Keep errors.Wrapf
			expectedOutput:      []string{`errors.Wrapf(originalErr, "%s"`, "tr.T(messages.Keys.ContextInfoS", "ctx"},
			forbiddenOutput:     []string{"errors.New", "errors.Wrap(", "%w"},
			description:         "errors.Wrapf should preserve function and maintain wrapping",
		},
		{
			name:                "complex fmt.Errorf with multiple format specifiers and %w",
			code:                `fmt.Errorf("failed to process %s: %w", filename, ioErr)`,
			expectErrorHandling: true,
			expectErrorWrap:     true,
			expectFuncChange:    false,
			expectedOutput:      []string{`fmt.Errorf("%s: %w"`, "tr.T(messages.Keys.FailedToProcessSW", "filename", "ioErr"},
			forbiddenOutput:     []string{"errors.New"},
			description:         "Complex error with format args and %w should handle correctly",
		},
		{
			name:                "fmt.Printf - not error handling",
			code:                `fmt.Printf("user logged in: %s", username)`,
			expectErrorHandling: false,
			expectErrorWrap:     false,
			expectFuncChange:    true, // Printf -> Print
			expectedOutput:      []string{"fmt.Print", "tr.T(messages.Keys.UserLoggedInS", "username"},
			forbiddenOutput:     []string{"errors.New", "fmt.Errorf", "%w"},
			description:         "Printf should use regular print transformation",
		},
		{
			name:                "log.Fatalf - not error creation",
			code:                `log.Fatalf("processing item %d", itemNum)`,
			expectErrorHandling: false,
			expectErrorWrap:     false,
			expectFuncChange:    true, // Fatalf -> Fatal
			expectedOutput:      []string{"log.Fatal", "tr.T(messages.Keys.ProcessingItemD", "itemNum"},
			forbiddenOutput:     []string{"errors.New", "fmt.Errorf", "%w"},
			description:         "log.Fatalf should use regular print transformation",
		},
		{
			name:                "fmt.Sprintf - direct replacement",
			code:                `msg := fmt.Sprintf("result: %v", value)`,
			expectErrorHandling: false,
			expectErrorWrap:     false,
			expectFuncChange:    true, // Becomes tr.T directly
			expectedOutput:      []string{"tr.T(messages.Keys.ResultV", "value"},
			forbiddenOutput:     []string{"errors.New", "fmt.Errorf", "fmt.Sprintf", "fmt.Print"},
			description:         "Sprintf should be replaced directly with tr.T",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullCode := `package main
import (
	"fmt"
	"log"
	"errors"
)

func test() {
	` + tt.code + `
}`

			transformer := NewFormatTransformer(stringMap)
			result, err := transformer.TransformFile("test.go", []byte(fullCode))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			resultStr := string(result)
			t.Logf("Input: %s", tt.code)
			t.Logf("Output: %s", strings.TrimSpace(strings.Split(resultStr, "func test()")[1]))

			// Check expected patterns
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected pattern not found: '%s'\nFull output:\n%s", expected, resultStr)
				}
			}

			// Check forbidden patterns
			for _, forbidden := range tt.forbiddenOutput {
				if strings.Contains(resultStr, forbidden) {
					t.Errorf("Forbidden pattern found: '%s'\nFull output:\n%s", forbidden, resultStr)
				}
			}

			// Verify error wrapping preservation
			if tt.expectErrorWrap {
				if !strings.Contains(resultStr, "%w") {
					t.Error("Expected %w to be preserved for error wrapping")
				}
			}

			// Verify error handling logic
			if tt.expectErrorHandling {
				// Should either have errors.New or preserve error function with proper handling
				hasErrorsNew := strings.Contains(resultStr, "errors.New")
				hasErrorWrap := strings.Contains(resultStr, "%w") 
				hasWrapf := strings.Contains(resultStr, "Wrapf")
				
				if !hasErrorsNew && !hasErrorWrap && !hasWrapf {
					t.Error("Error handling should result in errors.New, %w preservation, or Wrapf")
				}
			}
		})
	}
}

// TestErrorDetectionPrecedence tests that error detection takes precedence over other patterns
func TestErrorDetectionPrecedence(t *testing.T) {
	tests := []struct {
		name         string
		functionName string
		formatString string
		expectError  bool
		expectType   string
	}{
		{
			name:         "Errorf always detected as error regardless of pattern",
			functionName: "pkg.Errorf",
			formatString: "simple message",
			expectError:  true,
			expectType:   "Error",
		},
		{
			name:         "Wrapf always detected as wrap regardless of pattern", 
			functionName: "lib.Wrapf",
			formatString: "context message",
			expectError:  true,
			expectType:   "Wrapf",
		},
		{
			name:         "Printf never treated as error even with error-like message",
			functionName: "fmt.Printf",
			formatString: "error occurred: %s",
			expectError:  false,
			expectType:   "Print",
		},
		{
			name:         "Generic function ending in f not treated as error",
			functionName: "logger.Infof", 
			formatString: "info message: %s",
			expectError:  false,
			expectType:   "Print",
		},
	}

	detector := NewFormatDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &FormatCallInfo{
				FunctionName: tt.functionName,
				FormatString: tt.formatString,
			}

			transformType := detector.SuggestTransformation(info)
			isError := (transformType == "Error" || transformType == "Wrapf")

			if transformType != tt.expectType {
				t.Errorf("Expected transform type %s, got %s", tt.expectType, transformType)
			}

			if isError != tt.expectError {
				t.Errorf("Expected error detection %v, got %v", tt.expectError, isError)
			}
		})
	}
}