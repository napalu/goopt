package ast

import (
	"strings"
	"testing"
)

func TestSkipCommentsSimple(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		transformMode string
		expectSkipped bool // whether the string should be skipped
	}{
		{
			name: "line comment before var",
			input: `package main
import "fmt"
func test() {
	// i18n-skip
	msg := "should skip"
	fmt.Println(msg)
}`,
			transformMode: "all",
			expectSkipped: true,
		},
		{
			name: "block comment before var",
			input: `package main
import "fmt"
func test() {
	/* i18n-skip */
	msg := "should skip"
	fmt.Println(msg)
}`,
			transformMode: "all",
			expectSkipped: true,
		},
		{
			name: "line comment at end of line",
			input: `package main
import "fmt"
func test() {
	msg := "should skip" // i18n-skip
	fmt.Println(msg)
}`,
			transformMode: "all",
			expectSkipped: true,
		},
		{
			name: "block comment inline",
			input: `package main
import "fmt"
func test() {
	msg := "should skip" /* i18n-skip */
	fmt.Println(msg)
}`,
			transformMode: "all",
			expectSkipped: true,
		},
		{
			name: "no skip comment",
			input: `package main
import "fmt"
func test() {
	msg := "should translate"
	fmt.Println(msg)
}`,
			transformMode: "all",
			expectSkipped: false,
		},
		{
			name: "skip in function call",
			input: `package main
import "fmt"
func test() {
	fmt.Println("should skip") // i18n-skip
}`,
			transformMode: "user-facing",
			expectSkipped: true,
		},
		{
			name: "skip in multi-line call",
			input: `package main
import "fmt"
func test() {
	fmt.Printf(
		"should skip %s", // i18n-skip
		"arg")
}`,
			transformMode: "user-facing",
			expectSkipped: true,
		},
		{
			name: "case insensitive skip",
			input: `package main
import "fmt"
func test() {
	fmt.Println("should skip") // I18N-SKIP
}`,
			transformMode: "user-facing",
			expectSkipped: true,
		},
		{
			name: "skip with previous line comment",
			input: `package main
import "fmt"
func test() {
	// i18n-skip
	fmt.Println("should skip")
}`,
			transformMode: "user-facing",
			expectSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract the string to check
			var testString string
			if strings.Contains(tt.input, "should skip") {
				testString = "should skip"
			} else if strings.Contains(tt.input, "should translate") {
				testString = "should translate"
			} else {
				t.Fatal("Test input must contain either 'should skip' or 'should translate'")
			}

			// Handle format strings
			stringLiteral := `"` + testString + `"`
			if strings.Contains(tt.input, testString+" %s") {
				stringLiteral = `"` + testString + " %s" + `"`
			}

			// Create string map
			stringMap := map[string]string{
				stringLiteral: "messages.Keys.Test",
			}

			// Create transformer
			transformer := NewFormatTransformer(stringMap)
			transformer.SetTransformMode(tt.transformMode)

			// Transform
			result, err := transformer.TransformFile("test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			resultStr := string(result)

			// Check result
			containsOriginal := strings.Contains(resultStr, stringLiteral)
			containsTransformed := strings.Contains(resultStr, "tr.T(messages.Keys.Test")

			if tt.expectSkipped {
				if !containsOriginal || containsTransformed {
					t.Errorf("String should have been skipped but was transformed.\nOriginal present: %v\nTransformed present: %v\nResult:\n%s",
						containsOriginal, containsTransformed, resultStr)
				}
			} else {
				if containsOriginal || !containsTransformed {
					t.Errorf("String should have been transformed but was not.\nOriginal present: %v\nTransformed present: %v\nResult:\n%s",
						containsOriginal, containsTransformed, resultStr)
				}
			}
		})
	}
}

// Test that skip comments work in function calls
func TestSkipInFunctionCalls(t *testing.T) {
	input := `package main
import "fmt"
import "log"

func test() {
	// Test in various function calls
	fmt.Println("translate1", "skip1" /* i18n-skip */)
	
	// i18n-skip
	log.Printf("skip2")
	
	fmt.Printf("translate2 %s", "arg") 
	
	log.Fatal("skip3") // i18n-skip
}`

	stringMap := map[string]string{
		`"translate1"`:    "messages.Keys.Trans1",
		`"translate2 %s"`: "messages.Keys.Trans2",
		`"skip1"`:         "messages.Keys.Skip1",
		`"skip2"`:         "messages.Keys.Skip2",
		`"skip3"`:         "messages.Keys.Skip3",
	}

	transformer := NewFormatTransformer(stringMap)
	transformer.SetTransformMode("user-facing")

	result, err := transformer.TransformFile("test.go", []byte(input))
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	resultStr := string(result)

	// Check that skip strings are NOT transformed
	skipStrings := []string{`"skip1"`, `"skip2"`, `"skip3"`}
	for _, skip := range skipStrings {
		if !strings.Contains(resultStr, skip) {
			t.Errorf("Skip string %s was transformed but should have been skipped", skip)
		}
	}

	// Check that non-skip strings ARE transformed
	if !strings.Contains(resultStr, "tr.T(messages.Keys.Trans1)") {
		t.Error("translate1 was not transformed but should have been")
		t.Logf("Result:\n%s", resultStr)
	}
	if !strings.Contains(resultStr, "tr.T(messages.Keys.Trans2") {
		t.Error("translate2 with format specifier was not transformed but should have been")
		t.Logf("Result:\n%s", resultStr)
	}
}
