package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestErrorFunctionDetection tests that we correctly identify when to use error-specific handling
func TestErrorFunctionDetection(t *testing.T) {
	tests := []struct {
		name               string
		code               string
		expectedTransform  string // "Error", "Wrapf", "Print", etc.
		isErrorFunction    bool
		hasErrorWrap       bool
		shouldPreserveWrap bool
	}{
		{
			name:              "fmt.Errorf with %w",
			code:              `fmt.Errorf("failed: %w", err)`,
			expectedTransform: "Error",
			isErrorFunction:   true,
			hasErrorWrap:      true,
			shouldPreserveWrap: true,
		},
		{
			name:              "fmt.Errorf without %w",
			code:              `fmt.Errorf("simple error")`,
			expectedTransform: "Error",
			isErrorFunction:   true,
			hasErrorWrap:      false,
			shouldPreserveWrap: false,
		},
		{
			name:              "errors.Wrapf with format",
			code:              `errors.Wrapf(err, "context: %s", ctx)`,
			expectedTransform: "Wrapf",
			isErrorFunction:   true,
			hasErrorWrap:      false, // Wrapf doesn't use %w, it wraps by design
			shouldPreserveWrap: true,
		},
		{
			name:              "errors.Errorf (generic)",
			code:              `errors.Errorf("error: %v", val)`,
			expectedTransform: "Error",
			isErrorFunction:   true,
			hasErrorWrap:      false,
			shouldPreserveWrap: false,
		},
		{
			name:              "fmt.Printf (not error)",
			code:              `fmt.Printf("hello: %s", name)`,
			expectedTransform: "Print",
			isErrorFunction:   false,
			hasErrorWrap:      false,
			shouldPreserveWrap: false,
		},
		{
			name:              "log.Fatalf (not error creation)",
			code:              `log.Fatalf("fatal: %s", msg)`,
			expectedTransform: "Print",
			isErrorFunction:   false,
			hasErrorWrap:      false,
			shouldPreserveWrap: false,
		},
		{
			name:              "custom.Errorf (should be detected as error)",
			code:              `custom.Errorf("custom error: %v", val)`,
			expectedTransform: "Error",
			isErrorFunction:   true,
			hasErrorWrap:      false,
			shouldPreserveWrap: false,
		},
		{
			name:              "pkg.Wrapf (should be detected as wrap)",
			code:              `pkg.Wrapf(err, "wrapped: %s", ctx)`,
			expectedTransform: "Wrapf",
			isErrorFunction:   true,
			hasErrorWrap:      false,
			shouldPreserveWrap: true,
		},
		{
			name:              "fmt.Sprintf (string return, not error)",
			code:              `fmt.Sprintf("format: %s", val)`,
			expectedTransform: "Direct",
			isErrorFunction:   false,
			hasErrorWrap:      false,
			shouldPreserveWrap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the code
			fullCode := `package main
import (
	"fmt"
	"log"
	"errors"
)
func test() {
	` + tt.code + `
}`

			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", fullCode, 0)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			// Find the function call
			var call *ast.CallExpr
			ast.Inspect(node, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					// Skip if this is just a function name match (like "test")
					if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok {
							if ident.Name != "test" { // Skip the test() function call
								call = c
								return false
							}
						}
					} else if ident, ok := c.Fun.(*ast.Ident); ok {
						if ident.Name != "test" {
							call = c
							return false
						}
					}
				}
				return true
			})

			if call == nil {
				t.Fatal("Could not find function call in code")
			}

			// Test format detection
			detector := NewFormatDetector()
			formatInfo := detector.DetectFormatCall(call)

			if formatInfo == nil {
				t.Fatal("Format call not detected")
			}

			// Test transformation suggestion
			transformType := detector.SuggestTransformation(formatInfo)
			if transformType != tt.expectedTransform {
				t.Errorf("Expected transformation %s, got %s", tt.expectedTransform, transformType)
			}

			// Test error function identification
			isErrorFunc := (transformType == "Error" || transformType == "Wrapf")
			if isErrorFunc != tt.isErrorFunction {
				t.Errorf("Expected isErrorFunction=%v, got %v", tt.isErrorFunction, isErrorFunc)
			}

			// Test %w detection
			hasErrorWrap := strings.Contains(formatInfo.FormatString, "%w")
			if hasErrorWrap != tt.hasErrorWrap {
				t.Errorf("Expected hasErrorWrap=%v, got %v", tt.hasErrorWrap, hasErrorWrap)
			}

			// Test specific logic for each transform type
			switch transformType {
			case "Error":
				// Should be error creation function
				if !strings.Contains(strings.ToLower(formatInfo.FunctionName), "error") {
					t.Error("Error transform should be for error-related functions")
				}
			case "Wrapf":
				// Should be error wrapping function
				if !strings.Contains(strings.ToLower(formatInfo.FunctionName), "wrap") {
					t.Error("Wrapf transform should be for wrap-related functions")
				}
			case "Print":
				// Should not be error creation
				if strings.Contains(strings.ToLower(formatInfo.FunctionName), "error") {
					t.Error("Print transform should not be for error functions")
				}
			}
		})
	}
}

// TestErrorTransformationDecisionLogic tests the decision logic for error transformations
func TestErrorTransformationDecisionLogic(t *testing.T) {
	stringMap := map[string]string{
		`"connection failed: %w"`:    "messages.Keys.ConnectionFailedW",
		`"simple error"`:             "messages.Keys.SimpleError",
		`"context: %s"`:              "messages.Keys.ContextS",
		`"hello: %s"`:                "messages.Keys.HelloS",
	}

	tests := []struct {
		name            string
		input           string
		shouldTransform bool
		expectedPattern string // What pattern should appear in output
		shouldNotContain []string // Patterns that should NOT appear
	}{
		{
			name:            "fmt.Errorf with %w preserves wrapping",
			input:           `fmt.Errorf("connection failed: %w", err)`,
			shouldTransform: true,
			expectedPattern: `fmt.Errorf("%s: %w"`,
			shouldNotContain: []string{"errors.New"},
		},
		{
			name:            "fmt.Errorf without %w converts to errors.New",
			input:           `fmt.Errorf("simple error")`,
			shouldTransform: true,
			expectedPattern: `errors.New(tr.T(messages.Keys.SimpleError))`,
			shouldNotContain: []string{"fmt.Errorf", "%w"},
		},
		{
			name:            "errors.Wrapf preserves function name",
			input:           `errors.Wrapf(err, "context: %s", ctx)`,
			shouldTransform: true,
			expectedPattern: `errors.Wrapf(err, "%s", tr.T(messages.Keys.ContextS, ctx))`,
			shouldNotContain: []string{"errors.Wrap(", "errors.New"},
		},
		{
			name:            "fmt.Printf is not error handling",
			input:           `fmt.Printf("hello: %s", name)`,
			shouldTransform: true,
			expectedPattern: `fmt.Print(tr.T(messages.Keys.HelloS, name))`,
			shouldNotContain: []string{"errors.New", "%w", "fmt.Errorf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := `package main
import (
	"fmt"
	"errors"
)
func test() {
	` + tt.input + `
}`

			transformer := NewFormatTransformer(stringMap)
			result, err := transformer.TransformFile("test.go", []byte(code))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			resultStr := string(result)

			if tt.shouldTransform {
				if !strings.Contains(resultStr, tt.expectedPattern) {
					t.Errorf("Expected pattern not found:\n%s\n\nGot:\n%s", tt.expectedPattern, resultStr)
				}
			}

			for _, forbidden := range tt.shouldNotContain {
				if strings.Contains(resultStr, forbidden) {
					t.Errorf("Found forbidden pattern '%s' in result:\n%s", forbidden, resultStr)
				}
			}
		})
	}
}

// TestEdgeCasesInErrorDetection tests edge cases in error function detection
func TestEdgeCasesInErrorDetection(t *testing.T) {
	tests := []struct {
		name        string
		funcName    string
		shouldBeError bool
		expectedType string
	}{
		{
			name:          "std fmt.Errorf",
			funcName:      "fmt.Errorf",
			shouldBeError: true,
			expectedType:  "Error",
		},
		{
			name:          "errors pkg Errorf",
			funcName:      "errors.Errorf",
			shouldBeError: true,
			expectedType:  "Error",
		},
		{
			name:          "custom pkg Errorf",
			funcName:      "mypkg.Errorf",
			shouldBeError: true,
			expectedType:  "Error",
		},
		{
			name:          "method Errorf",
			funcName:      "logger.Errorf",
			shouldBeError: true,
			expectedType:  "Error",
		},
		{
			name:          "std errors.Wrapf",
			funcName:      "errors.Wrapf",
			shouldBeError: true,
			expectedType:  "Wrapf",
		},
		{
			name:          "custom Wrapf",
			funcName:      "pkg.Wrapf",
			shouldBeError: true,
			expectedType:  "Wrapf",
		},
		{
			name:          "Printf is not error",
			funcName:      "fmt.Printf",
			shouldBeError: false,
			expectedType:  "Print",
		},
		{
			name:          "Sprintf is not error",
			funcName:      "fmt.Sprintf",
			shouldBeError: false,
			expectedType:  "Direct",
		},
		{
			name:          "custom function ending in f",
			funcName:      "pkg.Logf",
			shouldBeError: false,
			expectedType:  "Print",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewFormatDetector()

			// Create a mock FormatCallInfo
			info := &FormatCallInfo{
				FunctionName: tt.funcName,
				FormatString: "test %s",
			}

			transformType := detector.SuggestTransformation(info)
			
			if transformType != tt.expectedType {
				t.Errorf("Expected %s, got %s", tt.expectedType, transformType)
			}

			isError := (transformType == "Error" || transformType == "Wrapf")
			if isError != tt.shouldBeError {
				t.Errorf("Expected shouldBeError=%v, got %v", tt.shouldBeError, isError)
			}
		})
	}
}