package ast

import (
	"strings"
	"testing"
)

func TestCommentPreservationOnly(t *testing.T) {
	tests := []struct {
		name                 string
		input                string
		expectedComments     []string
		unexpectedTransforms []string // strings that should NOT be transformed
		expectedTransforms   []string // strings that SHOULD be transformed
	}{
		{
			name: "preserve i18n-skip on assignment",
			input: `package main

func main() {
	query := "SELECT * FROM users" // i18n-skip
	msg := "User query executed"
}`,
			expectedComments:     []string{"// i18n-skip"},
			unexpectedTransforms: []string{"SELECT * FROM users"},
			expectedTransforms:   []string{"User query executed"},
		},
		{
			name: "preserve i18n-skip on var declaration",
			input: `package main

var (
	apiKey = "sk-12345" // i18n-skip
	welcomeMsg = "Welcome!"
)`,
			expectedComments:     []string{"// i18n-skip"},
			unexpectedTransforms: []string{"sk-12345"},
			expectedTransforms:   []string{"Welcome!"},
		},
		{
			name: "preserve multiple skip comments",
			input: `package main

import "fmt"

func process() {
	endpoint := "https://api.example.com" // i18n-skip
	fmt.Println("Connecting to endpoint")
	token := "Bearer xyz123" // i18n-skip
	fmt.Println("Authentication ready")
}`,
			expectedComments:     []string{"// i18n-skip"},
			unexpectedTransforms: []string{"https://api.example.com", "Bearer xyz123"},
			expectedTransforms:   []string{"Connecting to endpoint", "Authentication ready"},
		},
		{
			name: "const declarations remain unchanged",
			input: `package main

const (
	DefaultTimeout = "30s" // i18n-skip
	AppName = "My Application"
	Version = "1.0.0"
)`,
			expectedComments: []string{"// i18n-skip"},
			// All const strings should remain unchanged (can't use function calls in const)
			unexpectedTransforms: []string{"30s", "My Application", "1.0.0"},
			expectedTransforms:   []string{}, // No transformations in const context
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create string map
			stringMap := map[string]string{
				`"User query executed"`:    "app.extracted.user_query_executed",
				`"Welcome!"`:               "app.extracted.welcome",
				`"Connecting to endpoint"`: "app.extracted.connecting_to_endpoint",
				`"Authentication ready"`:   "app.extracted.authentication_ready",
				`"My Application"`:         "app.extracted.name",
				`"1.0.0"`:                  "app.extracted.version",
			}

			transformer := NewFormatTransformer(stringMap)
			transformer.SetTransformMode("all")

			result, err := transformer.TransformFile("test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("transformation failed: %v", err)
			}

			output := string(result)

			// Check that all expected comments are preserved
			for _, comment := range tt.expectedComments {
				if !strings.Contains(output, comment) {
					t.Errorf("Expected comment %q not found in output", comment)
				}
			}

			// Check that strings with skip comments were NOT transformed
			for _, str := range tt.unexpectedTransforms {
				if !strings.Contains(output, `"`+str+`"`) {
					t.Errorf("String %q was transformed when it should have been skipped", str)
				}
			}

			// Check that strings without skip comments WERE transformed (where possible)
			for _, str := range tt.expectedTransforms {
				if strings.Contains(output, `"`+str+`"`) {
					t.Errorf("String %q was not transformed when it should have been", str)
				}
				// Should contain tr.T call instead
				if !strings.Contains(output, "tr.T(messages.Keys.") {
					t.Errorf("Expected tr.T transformation not found for %q", str)
				}
			}
		})
	}
}

func TestSkipCommentPositioning(t *testing.T) {
	input := `package main

import "fmt"

func main() {
	// Regular comment
	msg1 := "Hello"
	
	msg2 := "World" // i18n-skip
	
	// Another regular comment
	fmt.Println(msg1, msg2)
}`

	stringMap := map[string]string{
		`"Hello"`: "app.extracted.hello",
		`"World"`: "app.extracted.world",
	}

	transformer := NewFormatTransformer(stringMap)
	transformer.SetTransformMode("all")

	result, err := transformer.TransformFile("test.go", []byte(input))
	if err != nil {
		t.Fatalf("transformation failed: %v", err)
	}

	output := string(result)

	// The i18n-skip comment should be preserved
	if !strings.Contains(output, "// i18n-skip") {
		t.Error("i18n-skip comment was not preserved")
	}

	// Check that "World" was not transformed
	if !strings.Contains(output, `"World"`) {
		t.Error("String with i18n-skip was transformed")
	}

	// Check that "Hello" was transformed
	if strings.Contains(output, `"Hello"`) {
		t.Error("String without i18n-skip was not transformed")
	}

	// Regular comments should also be preserved
	if !strings.Contains(output, "// Regular comment") {
		t.Error("Regular comments were not preserved")
	}
}
