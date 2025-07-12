package ast

import (
	"strings"
	"testing"
)

// TestCase represents a test case for format string transformation
type TestCase struct {
	name     string
	input    string
	expected string
	imports  []string // expected imports to be added
}

// Test cases covering all scenarios we need to handle
var transformTestCases = []TestCase{
	// Basic format functions
	{
		name:     "fmt.Printf with single arg",
		input:    `fmt.Printf("User %s logged in", username)`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.UserSLoggedIn, username))`,
		imports:  []string{"messages"},
	},
	{
		name:     "fmt.Printf with multiple args",
		input:    `fmt.Printf("User %s logged in at %v from %s", user, time.Now(), ip)`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.UserSLoggedInAtVFromS, user, time.Now(), ip))`,
		imports:  []string{"messages"},
	},
	{
		name:     "log.Printf",
		input:    `log.Printf("Error code: %d", 404)`,
		expected: `log.Print(tr.T(messages.Keys.App.Extracted.ErrorCodeD, 404))`,
		imports:  []string{"messages"},
	},
	{
		name:     "fmt.Sprintf returns string",
		input:    `msg := fmt.Sprintf("Welcome %s!", username)`,
		expected: `msg := tr.T(messages.Keys.App.Extracted.WelcomeS, username)`,
		imports:  []string{"messages"},
	},
	{
		name:     "fmt.Fprintf with writer",
		input:    `fmt.Fprintf(w, "Response: %s", data)`,
		expected: `fmt.Fprint(w, tr.T(messages.Keys.App.Extracted.ResponseS, data))`,
		imports:  []string{"messages"},
	},

	// Error handling
	{
		name:     "fmt.Errorf simple",
		input:    `err := fmt.Errorf("failed to connect")`,
		expected: `err := errors.New(tr.T(messages.Keys.App.Extracted.FailedToConnect))`,
		imports:  []string{"messages", "errors"},
	},
	{
		name:     "fmt.Errorf with format",
		input:    `err := fmt.Errorf("failed to connect to %s", host)`,
		expected: `err := errors.New(tr.T(messages.Keys.App.Extracted.FailedToConnectToS, host))`,
		imports:  []string{"messages", "errors"},
	},
	{
		name:     "fmt.Errorf with error wrapping",
		input:    `err := fmt.Errorf("connection failed: %w", originalErr)`,
		expected: `err := fmt.Errorf("%s: %w", tr.T(messages.Keys.App.Extracted.ConnectionFailed), originalErr)`,
		imports:  []string{"messages"},
	},
	{
		name:     "fmt.Errorf with mixed format and wrap",
		input:    `err := fmt.Errorf("failed to connect to %s: %w", host, originalErr)`,
		expected: `err := fmt.Errorf("%s: %w", tr.T(messages.Keys.App.Extracted.FailedToConnectToS, host), originalErr)`,
		imports:  []string{"messages"},
	},

	// Chained method calls (like zerolog)
	{
		name:     "zerolog simple Msgf",
		input:    `log.Info().Msgf("Server started on port %d", port)`,
		expected: `log.Info().Msg(tr.T(messages.Keys.App.Extracted.ServerStartedOnPortD, port))`,
		imports:  []string{"messages"},
	},
	{
		name:     "zerolog with fields",
		input:    `log.Error().Str("user", username).Msgf("Authentication failed for %s", username)`,
		expected: `log.Error().Str("user", username).Msg(tr.T(messages.Keys.App.Extracted.AuthenticationFailedForS, username))`,
		imports:  []string{"messages"},
	},
	{
		name:     "logrus WithFields",
		input:    `log.WithFields(log.Fields{"user": user}).Infof("User %s logged in", user)`,
		expected: `log.WithFields(log.Fields{"user": user}).Info(tr.T(messages.Keys.App.Extracted.UserSLoggedIn, user))`,
		imports:  []string{"messages"},
	},

	// Edge cases
	{
		name:     "string with no format specifiers",
		input:    `fmt.Printf("Hello world")`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.HelloWorld))`,
		imports:  []string{"messages"},
	},
	{
		name:     "nested function calls",
		input:    `fmt.Printf("Result: %v", processData(input))`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.ResultV, processData(input)))`,
		imports:  []string{"messages"},
	},
	{
		name:     "string concatenation should not transform",
		input:    `fmt.Println("Hello " + "world")`,
		expected: `fmt.Println("Hello " + "world")`,
		imports:  []string{},
	},
	{
		name:     "Printf with no args should not transform",
		input:    `fmt.Printf(getUserMessage())`,
		expected: `fmt.Printf(getUserMessage())`,
		imports:  []string{},
	},
	{
		name:     "multiple format strings in one statement",
		input:    `fmt.Printf("User: %s", user); fmt.Printf("Time: %v", time)`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.UserS, user)); fmt.Print(tr.T(messages.Keys.App.Extracted.TimeV, time))`,
		imports:  []string{"messages"},
	},
	{
		name:     "format string with percent literal",
		input:    `fmt.Printf("Progress: %d%%", percent)`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.ProgressD, percent))`,
		imports:  []string{"messages"},
	},
	{
		name:     "log.Fatalf should transform",
		input:    `log.Fatalf("Critical error: %v", err)`,
		expected: `log.Fatal(tr.T(messages.Keys.App.Extracted.CriticalErrorV, err))`,
		imports:  []string{"messages"},
	},
	{
		name:     "log.Panicf should transform",
		input:    `log.Panicf("Panic: %s", msg)`,
		expected: `log.Panic(tr.T(messages.Keys.App.Extracted.PanicS, msg))`,
		imports:  []string{"messages"},
	},

	// Should NOT transform
	{
		name:     "non-format function should transform if string is in map",
		input:    `fmt.Println("Hello world")`,
		expected: `fmt.Println(tr.T(messages.Keys.App.Extracted.HelloWorld))`,
		imports:  []string{"messages"},
	},
	{
		name:     "custom printf function",
		input:    `myLogger.Printf("Custom log: %s", data)`,
		expected: `myLogger.Printf("Custom log: %s", data)`,
		imports:  []string{},
	},
	{
		name:     "string not in our replacements map",
		input:    `fmt.Printf("Unknown string %s", data)`,
		expected: `fmt.Printf("Unknown string %s", data)`,
		imports:  []string{},
	},
}

// Edge cases to test error conditions
var errorTestCases = []TestCase{
	{
		name:     "empty format string",
		input:    `fmt.Printf("", arg)`,
		expected: `fmt.Printf("", arg)`, // Should not transform empty strings
		imports:  []string{},
	},
	{
		name:     "format string is a variable",
		input:    `fmt.Printf(formatStr, args...)`,
		expected: `fmt.Printf(formatStr, args...)`, // Cannot transform non-literals
		imports:  []string{},
	},
	{
		name:     "complex expression as format",
		input:    `fmt.Printf(getFormat(), args)`,
		expected: `fmt.Printf(getFormat(), args)`, // Cannot transform expressions
		imports:  []string{},
	},
}

// Test the transformation logic
func TestFormatTransformation(t *testing.T) {
	// Create a string map based on the test cases
	stringMap := map[string]string{
		`"User %s logged in"`:               "app.extracted.user_s_logged_in",
		`"User %s logged in at %v from %s"`: "app.extracted.user_s_logged_in_at_v_from_s",
		`"Error code: %d"`:                  "app.extracted.error_code_d",
		`"Welcome %s!"`:                     "app.extracted.welcome_s",
		`"Response: %s"`:                    "app.extracted.response_s",
		`"failed to connect"`:               "app.extracted.failed_to_connect",
		`"failed to connect to %s"`:         "app.extracted.failed_to_connect_to_s",
		`"failed to connect to %s: %w"`:     "app.extracted.failed_to_connect_to_s",
		`"connection failed: %w"`:           "app.extracted.connection_failed",
		`"Server started on port %d"`:       "app.extracted.server_started_on_port_d",
		`"Authentication failed for %s"`:    "app.extracted.authentication_failed_for_s",
		`"Hello world"`:                     "app.extracted.hello_world",
		`"Result: %v"`:                      "app.extracted.result_v",
		`"User: %s"`:                        "app.extracted.user_s",
		`"Time: %v"`:                        "app.extracted.time_v",
		`"Progress: %d%%"`:                  "app.extracted.progress_d",
		`"Critical error: %v"`:              "app.extracted.critical_error_v",
		`"Panic: %s"`:                       "app.extracted.panic_s",
	}

	// Create the transformer with the string map
	ft := NewFormatTransformer(stringMap)
	ft.SetMessagePackagePath("messages")
	ft.SetTransformMode("user-facing") // Only transform format functions and known user-facing functions

	for _, tc := range transformTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap the input in a simple Go program
			code := `package main
import "fmt"
import "log"
import "errors"

func main() {
	` + tc.input + `
}`

			// Transform the code
			result, err := ft.TransformFile("test.go", []byte(code))
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			// Extract just the transformed line from the result
			resultStr := string(result)
			lines := strings.Split(resultStr, "\n")
			var transformedLine string
			inImportBlock := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)

				// Track import blocks
				if strings.HasPrefix(trimmed, "import (") {
					inImportBlock = true
					continue
				}
				if inImportBlock && trimmed == ")" {
					inImportBlock = false
					continue
				}

				// Skip various non-code lines
				if trimmed != "" &&
					!inImportBlock &&
					!strings.HasPrefix(trimmed, "package") &&
					!strings.HasPrefix(trimmed, "import") &&
					!strings.HasPrefix(trimmed, "func") &&
					!strings.HasPrefix(trimmed, "//") &&
					trimmed != "}" && trimmed != "{" &&
					!strings.HasPrefix(trimmed, "var tr") &&
					!strings.Contains(trimmed, `"fmt"`) &&
					!strings.Contains(trimmed, `"messages"`) &&
					!strings.Contains(trimmed, `"github.com/napalu/goopt`) &&
					!strings.Contains(trimmed, `"errors"`) {
					transformedLine = trimmed
					break
				}
			}

			// Special handling for multiple statements test case
			if tc.name == "multiple format strings in one statement" {
				// For this test, we need to collect all transformed lines
				var codeLines []string
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					// Look specifically for fmt.Print calls
					if strings.HasPrefix(trimmed, "fmt.Print") {
						codeLines = append(codeLines, trimmed)
					}
				}
				transformedLine = strings.Join(codeLines, "; ")
			}

			// Check the transformation
			if transformedLine != tc.expected {
				t.Errorf("Transformation mismatch:\nGot:      %s\nExpected: %s", transformedLine, tc.expected)
			}

			// Check imports
			for _, expectedImport := range tc.imports {
				if !strings.Contains(resultStr, `"`+expectedImport+`"`) {
					t.Errorf("Missing expected import: %s", expectedImport)
				}
			}
		})
	}
}

// Test error cases where transformation should not happen
func TestFormatTransformationErrorCases(t *testing.T) {
	// Empty string map since these should not transform
	ft := NewFormatTransformer(map[string]string{})
	ft.SetMessagePackagePath("messages")

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap the input in a simple Go program
			code := `package main
import "fmt"

func main() {
	` + tc.input + `
}`

			// Transform the code
			result, err := ft.TransformFile("test.go", []byte(code))
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			// Extract just the line from the result
			resultStr := string(result)
			lines := strings.Split(resultStr, "\n")
			var transformedLine string
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "package") &&
					!strings.HasPrefix(trimmed, "import") &&
					!strings.HasPrefix(trimmed, "func") &&
					trimmed != "}" && trimmed != "{" {
					transformedLine = trimmed
					break
				}
			}

			// Check that no transformation occurred
			if transformedLine != tc.expected {
				t.Errorf("Unexpected transformation:\nGot:      %s\nExpected: %s", transformedLine, tc.expected)
			}
		})
	}
}

// Test helper to verify AST integrity
func TestASTIntegrity(t *testing.T) {
	// Test that transformed code is valid Go code
	testCode := `
package main

import (
	"fmt"
	"log"
	"errors"
)

func example() error {
	username := "john"
	fmt.Printf("User %s logged in", username)
	
	if err := doSomething(); err != nil {
		return fmt.Errorf("operation failed: %w", err)
	}
	
	log.Printf("Success for user %s", username)
	return nil
}
`
	// For now, just verify we can transform it without errors
	stringMap := map[string]string{
		`"User %s logged in"`:    "app.extracted.user_s_logged_in",
		`"operation failed: %w"`: "app.extracted.operation_failed_w",
		`"Success for user %s"`:  "app.extracted.success_for_user_s",
	}

	transformer := NewFormatTransformer(stringMap)
	_, err := transformer.TransformFile("test.go", []byte(testCode))
	if err != nil {
		t.Errorf("Failed to transform valid code: %v", err)
	}
}
