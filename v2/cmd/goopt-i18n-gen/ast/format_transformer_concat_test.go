package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestStringConcatenation tests handling of concatenated strings in format functions
func TestStringConcatenation(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldHandle bool
		description  string
	}{
		{
			name:         "simple concatenation no format",
			input:        `fmt.Printf("Hello " + "world")`,
			shouldHandle: false,
			description:  "No format specifiers, should not transform",
		},
		{
			name:         "concatenation with format specifiers",
			input:        `fmt.Printf("User: %s" + " Role: %s", name, role)`,
			shouldHandle: true,
			description:  "Should combine strings and transform",
		},
		{
			name:  "multi-line concatenation",
			input: `fmt.Printf("This is " +
				"a message with %s and " +
				"another %s", first, second)`,
			shouldHandle: true,
			description:  "Should handle multi-line concatenation",
		},
		{
			name:         "variable concatenation",
			input:        `fmt.Printf(prefix + "Message: %s" + suffix, data)`,
			shouldHandle: false,
			description:  "Contains non-literal parts, cannot transform",
		},
		{
			name:         "function call concatenation",
			input:        `fmt.Printf(getPrefix() + "Message: %s", data)`,
			shouldHandle: false,
			description:  "Contains function calls, cannot transform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input
			expr, err := parser.ParseExpr(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse expression: %v", err)
			}

			// Check if it's a call expression
			call, ok := expr.(*ast.CallExpr)
			if !ok {
				t.Fatal("Not a call expression")
			}

			// Analyze the first argument
			if len(call.Args) > 0 {
				canHandle := isTransformableFormatString(call.Args[0])
				if canHandle != tt.shouldHandle {
					t.Errorf("Expected shouldHandle=%v but got %v for: %s\n%s",
						tt.shouldHandle, canHandle, tt.input, tt.description)
				}
			}
		})
	}
}

// isTransformableFormatString checks if a format argument can be transformed
func isTransformableFormatString(expr ast.Expr) bool {
	str, isLiteral := extractFormatString(expr)
	// We can transform if it's a literal AND contains format specifiers
	return isLiteral && strings.Contains(str, "%")
}

// Helper to extract string from expression
func extractFormatString(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strings.Trim(e.Value, `"`), true
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			left, leftOk := extractFormatString(e.X)
			right, rightOk := extractFormatString(e.Y)
			if leftOk && rightOk {
				return left + right, true
			}
		}
	}
	return "", false
}

// TestConcatenationHandling tests the actual handling of concatenated strings
func TestConcatenationHandling(t *testing.T) {
	// Test that we can extract the full string from concatenation
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple concat",
			input:    `"Hello " + "world"`,
			expected: "Hello world",
		},
		{
			name:     "format concat",
			input:    `"User: %s" + " Role: %s"`,
			expected: "User: %s Role: %s",
		},
		{
			name:     "multi-line concat",
			input:    `"Line 1: %s\n" + "Line 2: %s\n"`,
			expected: "Line 1: %s\nLine 2: %s\n",
		},
		{
			name:     "triple concat",
			input:    `"Part 1 " + "Part 2 " + "Part 3"`,
			expected: "Part 1 Part 2 Part 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse expression: %v", err)
			}

			result := extractConcatenatedString(expr)
			if result != tt.expected {
				t.Errorf("Expected %q but got %q", tt.expected, result)
			}
		})
	}
}

// extractConcatenatedString extracts the full string from concatenated literals
func extractConcatenatedString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			// Properly unquote the string to handle escape sequences
			if unquoted, err := strconv.Unquote(e.Value); err == nil {
				return unquoted
			}
			// Fallback to simple trim if unquote fails
			return strings.Trim(e.Value, `"`)
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			// Recursively extract from both sides
			left := extractConcatenatedString(e.X)
			right := extractConcatenatedString(e.Y)
			if left != "" && right != "" {
				return left + right
			}
		}
	}
	return ""
}

// TestComplexFormatCases tests complex real-world scenarios
func TestComplexFormatCases(t *testing.T) {
	// Test with actual format transformer
	stringMap := map[string]string{
		`"User: %s Role: %s"`:                        "messages.Keys.App.Extracted.UserSRoleS",
		`"Failed to connect to %s"`:                  "messages.Keys.App.Extracted.FailedToConnectToS",
		`"This is a message with %s and another %s"`: "messages.Keys.App.Extracted.ThisIsAMessageWithSAndAnotherS",
		`"Line 1: %s\nLine 2: %s\n"`:                 "messages.Keys.App.Extracted.Line1SLine2S",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "concatenated format string",
			input: `package main
import "fmt"
func test() {
	fmt.Printf("User: %s" + " Role: %s", name, role)
}`,
			expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.UserSRoleS, name, role))`,
		},
		{
			name: "error with concatenation",
			input: `package main
import "fmt"
func test() error {
	return fmt.Errorf("Failed to " + "connect to " + "%s", host)
}`,
			expected: `return errors.New(tr.T(messages.Keys.App.Extracted.FailedToConnectToS, host))`,
		},
		{
			name: "multi-line concatenation",
			input: `package main
import "fmt"
func test() {
	fmt.Printf("This is " +
		"a message with %s and " +
		"another %s", first, second)
}`,
			expected: `fmt.Print(tr.T(messages.Keys.App.Extracted.ThisIsAMessageWithSAndAnotherS, first, second))`,
		},
	}

	transformer := NewFormatTransformer(stringMap)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformFile("test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			resultStr := string(result)
			if !strings.Contains(resultStr, tt.expected) {
				t.Errorf("Expected output to contain:\n%s\n\nGot:\n%s", tt.expected, resultStr)
			}
		})
	}
}