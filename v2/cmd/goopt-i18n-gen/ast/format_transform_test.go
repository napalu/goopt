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
		name: "fmt.Printf with single arg",
		input: `fmt.Printf("User %s logged in", username)`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.UserSLoggedIn, username))`,
		imports: []string{"messages"},
	},
	{
		name: "fmt.Printf with multiple args",
		input: `fmt.Printf("User %s logged in at %v from %s", user, time.Now(), ip)`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.UserSLoggedInAtVFromS, user, time.Now(), ip))`,
		imports: []string{"messages"},
	},
	{
		name: "log.Printf",
		input: `log.Printf("Error code: %d", 404)`,
		expected: `log.Print(tr.T(messages.Keys.App.Extracted.ErrorCodeD, 404))`,
		imports: []string{"messages"},
	},
	{
		name: "fmt.Sprintf returns string",
		input: `msg := fmt.Sprintf("Welcome %s!", username)`,
		expected: `msg := tr.T(messages.Keys.App.Extracted.WelcomeS, username)`,
		imports: []string{"messages"},
	},
	{
		name: "fmt.Fprintf with writer",
		input: `fmt.Fprintf(w, "Response: %s", data)`,
		expected: `fmt.Fprint(w, tr.T(messages.Keys.App.Extracted.ResponseS, data))`,
		imports: []string{"messages"},
	},

	// Error handling
	{
		name: "fmt.Errorf simple",
		input: `err := fmt.Errorf("failed to connect")`,
		expected: `err := errors.New(tr.T(messages.Keys.App.Extracted.FailedToConnect))`,
		imports: []string{"messages", "errors"},
	},
	{
		name: "fmt.Errorf with format",
		input: `err := fmt.Errorf("failed to connect to %s", host)`,
		expected: `err := errors.New(tr.T(messages.Keys.App.Extracted.FailedToConnectToS, host))`,
		imports: []string{"messages", "errors"},
	},
	{
		name: "fmt.Errorf with error wrapping",
		input: `err := fmt.Errorf("connection failed: %w", originalErr)`,
		expected: `err := fmt.Errorf("%s: %w", tr.T(messages.Keys.App.Extracted.ConnectionFailed), originalErr)`,
		imports: []string{"messages"},
	},
	{
		name: "fmt.Errorf with mixed format and wrap",
		input: `err := fmt.Errorf("failed to connect to %s: %w", host, originalErr)`,
		expected: `err := fmt.Errorf("%s: %w", tr.T(messages.Keys.App.Extracted.FailedToConnectToS, host), originalErr)`,
		imports: []string{"messages"},
	},

	// Chained method calls (like zerolog)
	{
		name: "zerolog simple Msgf",
		input: `log.Info().Msgf("Server started on port %d", port)`,
		expected: `log.Info().Msg(tr.T(messages.Keys.App.Extracted.ServerStartedOnPortD, port))`,
		imports: []string{"messages"},
	},
	{
		name: "zerolog with fields",
		input: `log.Error().Str("user", username).Msgf("Authentication failed for %s", username)`,
		expected: `log.Error().Str("user", username).Msg(tr.T(messages.Keys.App.Extracted.AuthenticationFailedForS, username))`,
		imports: []string{"messages"},
	},
	{
		name: "logrus WithFields",
		input: `log.WithFields(log.Fields{"user": user}).Infof("User %s logged in", user)`,
		expected: `log.WithFields(log.Fields{"user": user}).Info(tr.T(messages.Keys.App.Extracted.UserSLoggedIn, user))`,
		imports: []string{"messages"},
	},

	// Edge cases
	{
		name: "string with no format specifiers",
		input: `fmt.Printf("Hello world")`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.HelloWorld))`,
		imports: []string{"messages"},
	},
	{
		name: "nested function calls",
		input: `fmt.Printf("Result: %v", processData(input))`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.ResultV, processData(input)))`,
		imports: []string{"messages"},
	},
	{
		name: "string concatenation should not transform",
		input: `fmt.Println("Hello " + "world")`,
		expected: `fmt.Println("Hello " + "world")`,
		imports: []string{},
	},
	{
		name: "Printf with no args should not transform", 
		input: `fmt.Printf(getUserMessage())`,
		expected: `fmt.Printf(getUserMessage())`,
		imports: []string{},
	},
	{
		name: "multiple format strings in one statement",
		input: `fmt.Printf("User: %s", user); fmt.Printf("Time: %v", time)`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.UserS, user)); fmt.Print(tr.T(messages.Keys.App.Extracted.TimeV, time))`,
		imports: []string{"messages"},
	},
	{
		name: "format string with percent literal",
		input: `fmt.Printf("Progress: %d%%", percent)`,
		expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.ProgressD, percent))`,
		imports: []string{"messages"},
	},
	{
		name: "log.Fatalf should transform",
		input: `log.Fatalf("Critical error: %v", err)`,
		expected: `log.Fatal(tr.T(messages.Keys.App.Extracted.CriticalErrorV, err))`,
		imports: []string{"messages"},
	},
	{
		name: "log.Panicf should transform",
		input: `log.Panicf("Panic: %s", msg)`,
		expected: `log.Panic(tr.T(messages.Keys.App.Extracted.PanicS, msg))`,
		imports: []string{"messages"},
	},

	// Should NOT transform
	{
		name: "non-format function",
		input: `fmt.Println("Hello world")`,
		expected: `fmt.Println("Hello world")`,
		imports: []string{},
	},
	{
		name: "custom printf function",
		input: `myLogger.Printf("Custom log: %s", data)`,
		expected: `myLogger.Printf("Custom log: %s", data)`,
		imports: []string{},
	},
	{
		name: "string not in our replacements map",
		input: `fmt.Printf("Unknown string %s", data)`,
		expected: `fmt.Printf("Unknown string %s", data)`,
		imports: []string{},
	},
}

// Edge cases to test error conditions
var errorTestCases = []TestCase{
	{
		name: "empty format string",
		input: `fmt.Printf("", arg)`,
		expected: `fmt.Printf("", arg)`, // Should not transform empty strings
		imports: []string{},
	},
	{
		name: "format string is a variable",
		input: `fmt.Printf(formatStr, args...)`,
		expected: `fmt.Printf(formatStr, args...)`, // Cannot transform non-literals
		imports: []string{},
	},
	{
		name: "complex expression as format",
		input: `fmt.Printf(getFormat(), args)`,
		expected: `fmt.Printf(getFormat(), args)`, // Cannot transform expressions
		imports: []string{},
	},
}

// Test the transformation logic
func TestFormatTransformation(t *testing.T) {
	// This will test our AST transformation implementation
	for _, tc := range transformTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// TODO: Implement the actual test once we have the transformer
			// result := transformCode(tc.input)
			// assert.Equal(t, tc.expected, result)
			// assert.Equal(t, tc.imports, result.RequiredImports)
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
		`"User %s logged in"`:    "messages.Keys.App.Extracted.UserSLoggedIn",
		`"operation failed: %w"`: "messages.Keys.App.Extracted.OperationFailedW",
		`"Success for user %s"`:  "messages.Keys.App.Extracted.SuccessForUserS",
	}
	
	transformer := NewFormatTransformer(stringMap)
	_, err := transformer.TransformFile("test.go", []byte(testCode))
	if err != nil {
		t.Errorf("Failed to transform valid code: %v", err)
	}
}

// Benchmarks to ensure performance
func BenchmarkTransformation(b *testing.B) {
	// TODO: Benchmark the transformation of a large file
}

// Helper function to normalize whitespace for comparison
func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var normalized []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, "\n")
}