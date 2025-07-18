package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/common"
	"github.com/napalu/goopt/v2/i18n"
)

// TestStringInUserFacingFunctionDetection tests that we correctly detect strings inside functions that commonly display user-facing text
// This includes both format functions (Printf, Errorf) and regular logging/display functions (Print, Msg, Info)
func TestStringInUserFacingFunctionDetection(t *testing.T) {
	testCode := `package main

import (
	"fmt"
	"log"
	"errors"
)

type Logger struct{}
func (l *Logger) Info() *Logger { return l }
func (l *Logger) Error() *Logger { return l }
func (l *Logger) Err(error) *Logger { return l }
func (l *Logger) Msg(string) {}
func (l *Logger) Msgf(string, ...interface{}) {}
func (l *Logger) Str(string, string) *Logger { return l }

func main() {
	logger := &Logger{}
	
	// Format functions - should be detected
	fmt.Printf("Hello %s", "world")
	fmt.Sprintf("Count: %d", 42)
	fmt.Fprintf(w, "Writing %d bytes", count)
	fmt.Errorf("Error: %v", err)
	
	// Log format functions - should be detected
	log.Printf("Server started on port %d", port)
	log.Fatalf("Fatal error: %s", msg)
	
	// Errors package - should be detected
	errors.Errorf("Failed to parse: %s", input)
	errors.Wrapf(err, "context: %s", ctx)
	
	// Chained logging - should be detected
	logger.Info().Msg("Application started")
	logger.Error().Err(err).Msgf("Failed to process %s", file)
	logger.Info().Str("key", "value").Msg("With metadata")
	
	// Regular strings - should NOT be detected
	msg := "Regular string"
	fmt.Println(msg)
	fmt.Print("No format specifier")
	
	// Nested calls - format strings should be detected
	wrapper(fmt.Sprintf("Nested %s", "format"))
}
`

	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, testFile, testCode, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test code: %v", err)
	}

	// Create replacer
	bundle := i18n.NewEmptyBundle()
	config := &common.TransformationConfig{
		Translator:    bundle,
		TransformMode: "user-facing",
	}
	tr := &TransformationReplacer{
		config: config,
		keyMap: make(map[string]string),
	}

	// Track which strings are detected as being in format functions
	detectedStrings := make(map[string]bool)
	notDetectedStrings := make(map[string]bool)

	// Walk the AST with parent tracking
	tr.walkASTWithParents(node, func(n ast.Node, parents []ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			tr.parentStack = parents
			value := strings.Trim(lit.Value, `"`)

			if tr.isInUserFacingFunction(lit) {
				detectedStrings[value] = true
			} else {
				notDetectedStrings[value] = true
			}
		}
		return true
	})

	// Verify format strings were detected
	expectedDetected := []string{
		"Hello %s",
		"Count: %d",
		"Writing %d bytes",
		"Error: %v",
		"Server started on port %d",
		"Fatal error: %s",
		"Failed to parse: %s",
		"context: %s",
		"Failed to process %s",
		"Application started",
		"With metadata",
		"key", // First arg to Str()
		"Nested %s",
	}

	for _, expected := range expectedDetected {
		if !detectedStrings[expected] {
			t.Errorf("String '%s' should have been detected as in format function", expected)
		}
	}

	// Verify non-format strings were NOT detected
	expectedNotDetected := []string{
		"Regular string",
		"No format specifier",
		"world",  // argument to Printf, not format string
		"value",  // second arg to Str()
		"format", // argument to Sprintf
	}

	for _, expected := range expectedNotDetected {
		if detectedStrings[expected] {
			t.Errorf("String '%s' should NOT have been detected as in format function", expected)
		}
	}
}

// TestFormatFunctionEdgeCases tests edge cases for format function detection
func TestFormatFunctionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected map[string]bool // string -> should be detected
	}{
		{
			name: "slog style logging",
			code: `package main
import "log/slog"
func test() {
	slog.Info("Server started", "port", 8080)
	slog.Error("Failed to connect", "error", err, "retry", 3)
}`,
			expected: map[string]bool{
				"Server started":    true, // slog.Info IS a user-facing function
				"Failed to connect": true, // slog.Error IS a user-facing function
				"port":              true, // argument to a user-facing function
				"error":             true, // argument to a user-facing function
				"retry":             true, // argument to a user-facing function
			},
		},
		{
			name: "complex chaining",
			code: `package main
func test() {
	logger.With().Str("request_id", "rid123").Logger().Info().Msg("Processing request")
	logger.WithLevel(ErrorLevel).Err(err).Str("file", "main.go").Msgf("Failed: %s", reason)
}`,
			expected: map[string]bool{
				"request_id":         true,  // Str() is in a chained logging call
				"Processing request": true,  // Msg() IS a user-facing function
				"file":               true,  // Str() is in a chained logging call
				"Failed: %s":         true,  // Msgf() IS a format function
				"rid123":             false, // Str() second arg, not translatable
				"main.go":            false, // Str() second arg, not translatable
			},
		},
		{
			name: "method vs function",
			code: `package main
type Custom struct{}
func (c *Custom) Printf(format string, args ...interface{}) {}
func test() {
	c := &Custom{}
	c.Printf("Custom format %s", "arg")
	Printf("Not a method %s", "arg")
}`,
			expected: map[string]bool{
				"Custom format %s": false, // Custom Printf, not fmt.Printf
				"Not a method %s":  false, // Unknown function Printf
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the code
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			// Create replacer
			bundle := i18n.NewEmptyBundle()
			config := &common.TransformationConfig{
				Translator:    bundle,
				TransformMode: "user-facing",
			}
			tr := &TransformationReplacer{
				config: config,
				keyMap: make(map[string]string),
			}

			// Track detections
			detections := make(map[string]bool)

			// Walk AST
			tr.walkASTWithParents(node, func(n ast.Node, parents []ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					tr.parentStack = parents
					value := strings.Trim(lit.Value, `"`)
					detections[value] = tr.isInUserFacingFunction(lit)
				}
				return true
			})

			// Verify expectations
			for str, shouldDetect := range tt.expected {
				if detected, ok := detections[str]; ok {
					if detected != shouldDetect {
						if shouldDetect {
							t.Errorf("String '%s' should have been detected as in format function", str)
						} else {
							t.Errorf("String '%s' should NOT have been detected as in format function", str)
						}
					}
				} else {
					t.Errorf("String '%s' was not found in the code", str)
				}
			}
		})
	}
}

// TestParentStackTracking verifies that walkASTWithParents correctly maintains parent stack
func TestParentStackTracking(t *testing.T) {
	code := `package main
func outer() {
	inner(func() {
		deep := "nested string"
		println(deep)
	})
}`

	// Parse
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	bundle := i18n.NewEmptyBundle()
	config := &common.TransformationConfig{
		Translator:    bundle,
		TransformMode: "user-facing",
	}
	tr := &TransformationReplacer{
		config: config,
	}

	foundNestedString := false
	tr.walkASTWithParents(node, func(n ast.Node, parents []ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			value := strings.Trim(lit.Value, `"`)
			if value == "nested string" {
				foundNestedString = true
				// Should have multiple parents: File > FuncDecl > BlockStmt > ExprStmt > CallExpr > FuncLit > BlockStmt > AssignStmt
				if len(parents) < 4 {
					t.Errorf("Expected at least 4 parents for nested string, got %d", len(parents))
				}
			}
		}
		return true
	})

	if !foundNestedString {
		t.Error("Failed to find nested string in AST walk")
	}
}

// BenchmarkIsInUserFacingFunction tests performance
func BenchmarkIsInUserFacingFunction(b *testing.B) {
	bundle := i18n.NewEmptyBundle()
	config := &common.TransformationConfig{
		Translator:    bundle,
		TransformMode: "user-facing",
	}
	tr := &TransformationReplacer{
		config: config,
		// Simulate a realistic parent stack
		parentStack: []ast.Node{
			&ast.File{},
			&ast.FuncDecl{},
			&ast.BlockStmt{},
			&ast.ExprStmt{},
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   &ast.Ident{Name: "fmt"},
					Sel: &ast.Ident{Name: "Printf"},
				},
			},
		},
	}

	lit := &ast.BasicLit{
		Kind:  token.STRING,
		Value: `"Hello %s"`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tr.isInUserFacingFunction(lit)
	}
}

func TestConvertKeyToASTFormat(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		packagePath string
		expected    string
	}{
		// Basic cases
		{
			name:        "simple key",
			key:         "app.extracted.hello_world",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.HelloWorld",
		},
		{
			name:        "with package path",
			key:         "app.extracted.error_message",
			packagePath: "./messages",
			expected:    "messages.Keys.App.Extracted.ErrorMessage",
		},
		{
			name:        "full module path",
			key:         "app.test.sample",
			packagePath: "github.com/user/project/messages",
			expected:    "messages.Keys.App.Test.Sample",
		},

		// Numeric cases - critical for our fix
		{
			name:        "all numeric key",
			key:         "app.extracted.n0000_00_00_00_00_00",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.N00000000000000",
		},
		{
			name:        "date format numeric",
			key:         "app.extracted.n2023_12_25_00_00_00",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.N20231225000000",
		},
		{
			name:        "simple numeric",
			key:         "app.extracted.n12345",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.N12345",
		},
		{
			name:        "mixed numeric",
			key:         "app.extracted.error_404__not_found",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.ErrorN404NotFound",
		},
		{
			name:        "numeric in middle",
			key:         "app.extracted.order__12345_processed",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.OrderN12345Processed",
		},
		{
			name:        "leading number",
			key:         "app.extracted.n123_items_found",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.N123ItemsFound",
		},

		// Format string cases
		{
			name:        "format string single",
			key:         "app.extracted.hello__s",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.HelloS",
		},
		{
			name:        "format string multiple",
			key:         "app.extracted.removed_child_group__s_from_group__s2",
			packagePath: "messages",
			expected:    "messages.Keys.App.Extracted.RemovedChildGroupSFromGroupS2",
		},

		// Edge cases
		{
			name:        "empty key part",
			key:         "app..test",
			packagePath: "messages",
			expected:    "messages.Keys.App..Test",
		},
		{
			name:        "single part key",
			key:         "simple",
			packagePath: "messages",
			expected:    "messages.Keys.Simple",
		},
		{
			name:        "relative package path",
			key:         "app.test.key",
			packagePath: "../messages",
			expected:    "messages.Keys.App.Test.Key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &TransformationReplacer{
				config: &common.TransformationConfig{
					PackagePath: tt.packagePath,
				},
			}

			result := sr.convertKeyToASTFormat(tt.key)
			if result != tt.expected {
				t.Errorf("convertKeyToASTFormat(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

// TestConvertKeyToASTFormatNumericConsistency verifies that numeric keys are
// converted consistently with the generate command
func TestConvertKeyToASTFormatNumericConsistency(t *testing.T) {
	// These are the actual keys from our test case
	numericKeys := []struct {
		key      string
		expected string
	}{
		{
			key:      "app.extracted.n0000_00_00_00_00_00",
			expected: "messages.Keys.App.Extracted.N00000000000000",
		},
		{
			key:      "app.extracted.n2023_12_25_00_00_00",
			expected: "messages.Keys.App.Extracted.N20231225000000",
		},
		{
			key:      "app.extracted.n12345",
			expected: "messages.Keys.App.Extracted.N12345",
		},
		{
			key:      "app.extracted.n123_items_found",
			expected: "messages.Keys.App.Extracted.N123ItemsFound",
		},
		{
			key:      "app.extracted.error_404__not_found",
			expected: "messages.Keys.App.Extracted.ErrorN404NotFound",
		},
		{
			key:      "app.extracted.order__12345_processed",
			expected: "messages.Keys.App.Extracted.OrderN12345Processed",
		},
	}

	sr := &TransformationReplacer{
		config: &common.TransformationConfig{
			PackagePath: "messages",
		},
	}

	for _, tt := range numericKeys {
		t.Run(tt.key, func(t *testing.T) {
			result := sr.convertKeyToASTFormat(tt.key)
			if result != tt.expected {
				t.Errorf("convertKeyToASTFormat(%q) = %q, want %q", tt.key, result, tt.expected)
			}

			// Verify that the last part (after the last dot) starts with a capital letter
			// This is required for Go exported identifiers
			lastDotIdx := len(result) - 1
			for i := len(result) - 1; i >= 0; i-- {
				if result[i] == '.' {
					lastDotIdx = i
					break
				}
			}

			if lastDotIdx < len(result)-1 {
				lastPart := result[lastDotIdx+1:]
				if len(lastPart) > 0 && (lastPart[0] < 'A' || lastPart[0] > 'Z') {
					t.Errorf("Last part of %q doesn't start with capital letter: %q", result, lastPart)
				}
			}
		})
	}
}
