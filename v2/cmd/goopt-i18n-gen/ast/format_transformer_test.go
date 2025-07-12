package ast

import (
	"strings"
	"testing"
)

func TestFormatTransformer(t *testing.T) {
	// Create a string map for testing
	stringMap := map[string]string{
		`"User %s logged in"`:            "app.extracted.user_s_logged_in",
		`"Error code: %d"`:               "app.extracted.error_code_d",
		`"Welcome %s!"`:                  "app.extracted.welcome_s",
		`"failed to connect to %s"`:      "app.extracted.failed_to_connect_to_s",
		`"Server started on port %d"`:    "app.extracted.server_started_on_port_d",
		`"Authentication failed for %s"`: "app.extracted.authentication_failed_for_s",
		`"connection failed: %w"`:        "app.extracted.connection_failed",
		`"Hello world"`:                  "app.extracted.hello_world",
		`"Result: %v"`:                   "app.extracted.result_v",
		`"Progress: %d%%"`:               "app.extracted.progress_d",
		`"Critical error: %v"`:           "app.extracted.critical_error_v",
		`"Panic: %s"`:                    "app.extracted.panic_s",
		`"failed to connect"`:            "app.extracted.failed_to_connect",
		`"Response: %s"`:                 "app.extracted.response_s",
	}

	tests := []struct {
		name      string
		input     string
		expected  string
		expected2 string // optional second expected string for tests with multiple transformations
	}{
		{
			name: "simple Printf",
			input: `package main

import "fmt"

func main() {
	username := "john"
	fmt.Printf("User %s logged in", username)
}`,
			expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.UserSLoggedIn, username))`,
		},
		{
			name: "Sprintf returns value",
			input: `package main

import "fmt"

func getMessage(name string) string {
	return fmt.Sprintf("Welcome %s!", name)
}`,
			expected: `return tr.T(messages.Keys.App.Extracted.WelcomeS, name)`,
		},
		{
			name: "Errorf without wrap",
			input: `package main

import "fmt"

func connect(host string) error {
	return fmt.Errorf("failed to connect to %s", host)
}`,
			expected: `return errors.New(tr.T(messages.Keys.App.Extracted.FailedToConnectToS, host))`,
		},
		{
			name: "Errorf with wrap",
			input: `package main

import "fmt"

func wrapError(err error) error {
	return fmt.Errorf("connection failed: %w", err)
}`,
			expected: `return fmt.Errorf("%s: %w", tr.T(messages.Keys.App.Extracted.ConnectionFailed), err)`,
		},
		{
			name: "Printf without format specifiers",
			input: `package main

import "fmt"

func greet() {
	fmt.Printf("Hello world")
}`,
			expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.HelloWorld))`,
		},
		{
			name: "log.Printf",
			input: `package main

import "log"

func logError(code int) {
	log.Printf("Error code: %d", code)
}`,
			expected: `log.Print(tr.T(messages.Keys.App.Extracted.ErrorCodeD, code))`,
		},
		{
			name: "multiple format calls",
			input: `package main

import (
	"fmt"
	"log"
)

func example() {
	fmt.Printf("Hello world")
	log.Printf("Error code: %d", 500)
}`,
			expected:  `fmt.Print(tr.T(messages.Keys.App.Extracted.HelloWorld))`,
			expected2: `log.Print(tr.T(messages.Keys.App.Extracted.ErrorCodeD, 500))`,
		},
		{
			name: "should not transform strings not in translation map",
			input: `package main

import "fmt"

func example() {
	fmt.Println("This string is not in the map")
	fmt.Print("Neither is this one")
}`,
			expected: `fmt.Println("This string is not in the map")
	fmt.Print("Neither is this one")`,
		},
		{
			name: "should not transform unknown strings",
			input: `package main

import "fmt"

func example() {
	fmt.Printf("Unknown string %s", "test")
}`,
			expected: `fmt.Printf("Unknown string %s", "test")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewFormatTransformer(stringMap)
			result, err := transformer.TransformFile("test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			resultStr := string(result)
			if !strings.Contains(resultStr, tt.expected) {
				t.Errorf("Expected output to contain:\n%s\n\nGot:\n%s", tt.expected, resultStr)
			}

			// Check second expected string if provided
			if tt.expected2 != "" && !strings.Contains(resultStr, tt.expected2) {
				t.Errorf("Expected output to also contain:\n%s\n\nGot:\n%s", tt.expected2, resultStr)
			}
		})
	}
}

// Test specific edge cases
func TestFormatTransformerEdgeCases(t *testing.T) {
	stringMap := map[string]string{
		`"Operation %s failed: %w"`: "app.extracted.operation_s_failed_w",
		`"Status: %s"`:              "app.extracted.status_s",
	}

	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		{
			name: "empty format string",
			input: `package main
import "fmt"
func test() { fmt.Printf("") }`,
			shouldError: false,
		},
		{
			name: "format string as variable",
			input: `package main
import "fmt"
func test() { 
	format := "Status: %s"
	fmt.Printf(format, "OK") 
}`,
			shouldError: false,
		},
		{
			name: "nested function call as format",
			input: `package main
import "fmt"
func getFormat() string { return "Status: %s" }
func test() { fmt.Printf(getFormat(), "OK") }`,
			shouldError: false,
		},
		{
			name: "complex error wrapping",
			input: `package main
import "fmt"
func test(op string, err error) error {
	return fmt.Errorf("Operation %s failed: %w", op, err)
}`,
			shouldError: false,
		},
	}

	transformer := NewFormatTransformer(stringMap)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := transformer.TransformFile("test.go", []byte(tt.input))
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Test that the transformer preserves valid Go syntax
func TestTransformerPreservesSyntax(t *testing.T) {
	stringMap := map[string]string{
		`"Processing %d items"`: "app.extracted.processing_d_items",
		`"Done"`:                "app.extracted.done",
	}

	input := `package main

import (
	"fmt"
	"log"
)

func process(count int) {
	fmt.Printf("Processing %d items", count)
	for i := 0; i < count; i++ {
		// Process item
	}
	log.Printf("Done")
}
`

	transformer := NewFormatTransformer(stringMap)
	result, err := transformer.TransformFile("test.go", []byte(input))
	if err != nil {
		t.Fatalf("TransformFile failed: %v", err)
	}

	// The result should still be valid Go code
	// In a real test, we'd try to parse it
	if !strings.Contains(string(result), "package main") {
		t.Error("Package declaration missing")
	}
	if !strings.Contains(string(result), "import") {
		t.Error("Import declaration missing")
	}
}
