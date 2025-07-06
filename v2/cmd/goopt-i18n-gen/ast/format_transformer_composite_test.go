package ast

import (
	"strings"
	"testing"
)

func TestFormatTransformerCompositeLiterals(t *testing.T) {
	// Test cases for composite literals with inline i18n-todo comments
	stringMap := map[string]string{
		`"Hello"`:     "messages.Keys.App.Extracted.Hello",
		`"World"`:     "messages.Keys.App.Extracted.World",
		`"Goodbye"`:   "messages.Keys.App.Extracted.Goodbye",
		`"Welcome"`:   "messages.Keys.App.Extracted.Welcome",
		`"Item %d"`:   "messages.Keys.App.Extracted.ItemD",
		`"Error: %s"`: "messages.Keys.App.Extracted.ErrorS",
	}

	tests := []struct {
		name             string
		transformMode    string
		input            string
		expected         string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:          "array with inline i18n-todo comments",
			transformMode: "with-comments",
			input: `package main

func main() {
	messages := []string{
		"Hello",   // i18n-todo: tr.T(messages.Keys.App.Hello)
		"World",   // i18n-todo: tr.T(messages.Keys.App.World)
		"Goodbye", // i18n-todo: tr.T(messages.Keys.App.Goodbye)
	}
}`,
			shouldContain: []string{
				`tr.T(messages.Keys.App.Extracted.Hello)`,
				`tr.T(messages.Keys.App.Extracted.World)`,
				`tr.T(messages.Keys.App.Extracted.Goodbye)`,
			},
		},
		{
			name:          "map literal with inline i18n-todo comments",
			transformMode: "with-comments",
			input: `package main

func main() {
	labels := map[string]string{
		"greeting": "Hello",   // i18n-todo: tr.T(messages.Keys.App.Hello)
		"farewell": "Goodbye", // i18n-todo: tr.T(messages.Keys.App.Goodbye)
	}
}`,
			shouldContain: []string{
				`"greeting": tr.T(messages.Keys.App.Extracted.Hello)`,
				`"farewell": tr.T(messages.Keys.App.Extracted.Goodbye)`,
			},
		},
		{
			name:          "slice literal with format strings and i18n-todo",
			transformMode: "with-comments",
			input: `package main

func main() {
	errors := []string{
		"Item %d",   // i18n-todo: tr.T(messages.Keys.App.ItemD)
		"Error: %s", // i18n-todo: tr.T(messages.Keys.App.ErrorS)
	}
}`,
			shouldContain: []string{
				`tr.T(messages.Keys.App.Extracted.ItemD)`,
				`tr.T(messages.Keys.App.Extracted.ErrorS)`,
			},
		},
		{
			name:          "mixed composite with and without i18n-todo",
			transformMode: "with-comments",
			input: `package main

func main() {
	data := []string{
		"Hello",     // i18n-todo: tr.T(messages.Keys.App.Hello)
		"NotInMap",  // This string is not in the translation map
		"Welcome",   // i18n-todo: tr.T(messages.Keys.App.Welcome)
	}
}`,
			shouldContain: []string{
				`tr.T(messages.Keys.App.Extracted.Hello)`,
				`tr.T(messages.Keys.App.Extracted.Welcome)`,
			},
			shouldNotContain: []string{
				`tr.T(messages.Keys.App.Extracted.NotInMap)`,
			},
		},
		{
			name:          "nested composite literals",
			transformMode: "with-comments",
			input: `package main

func main() {
	config := map[string][]string{
		"messages": {
			"Hello",   // i18n-todo: tr.T(messages.Keys.App.Hello)
			"World",   // i18n-todo: tr.T(messages.Keys.App.World)
		},
		"errors": {
			"Error: %s", // i18n-todo: tr.T(messages.Keys.App.ErrorS)
		},
	}
}`,
			shouldContain: []string{
				`tr.T(messages.Keys.App.Extracted.Hello)`,
				`tr.T(messages.Keys.App.Extracted.World)`,
				`tr.T(messages.Keys.App.Extracted.ErrorS)`,
			},
		},
		{
			name:          "struct literal with i18n-todo",
			transformMode: "with-comments",
			input: `package main

type Message struct {
	Title string
	Body  string
}

func main() {
	msg := Message{
		Title: "Hello",   // i18n-todo: tr.T(messages.Keys.App.Hello)
		Body:  "Welcome", // i18n-todo: tr.T(messages.Keys.App.Welcome)
	}
}`,
			shouldContain: []string{
				`Title: tr.T(messages.Keys.App.Extracted.Hello)`,
				`Body: tr.T(messages.Keys.App.Extracted.Welcome)`,
			},
		},
		{
			name:          "all-marked mode should transform both user-facing and i18n-todo",
			transformMode: "all-marked",
			input: `package main

import "fmt"

func main() {
	// This should be transformed (user-facing)
	fmt.Printf("Item %d", 42)
	
	// This should also be transformed (i18n-todo)
	msg := "Hello" // i18n-todo: tr.T(messages.Keys.App.Hello)
}`,
			shouldContain: []string{
				`fmt.Print(tr.T(messages.Keys.App.Extracted.ItemD, 42))`,
				`msg := tr.T(messages.Keys.App.Extracted.Hello)`,
			},
		},
		{
			name:          "user-facing mode ignores i18n-todo in composites",
			transformMode: "user-facing",
			input: `package main

import "fmt"

func main() {
	// This should be transformed (user-facing)
	fmt.Printf("Item %d", 42)
	
	// This should NOT be transformed (only i18n-todo)
	messages := []string{
		"Hello", // i18n-todo: tr.T(messages.Keys.App.Hello)
	}
}`,
			shouldContain: []string{
				`fmt.Print(tr.T(messages.Keys.App.Extracted.ItemD, 42))`,
			},
			shouldNotContain: []string{
				`tr.T(messages.Keys.App.Extracted.Hello)`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewFormatTransformer(stringMap)
			transformer.SetTransformMode(tt.transformMode)

			result, err := transformer.TransformFile("test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			resultStr := string(result)

			// Check expected contents
			for _, expected := range tt.shouldContain {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected output to contain:\n%s\n\nGot:\n%s", expected, resultStr)
				}
			}

			// Check that certain strings should NOT be present
			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(resultStr, notExpected) {
					t.Errorf("Expected output NOT to contain:\n%s\n\nGot:\n%s", notExpected, resultStr)
				}
			}

			// Verify imports were added when transformations occurred
			if len(tt.shouldContain) > 0 {
				if !strings.Contains(resultStr, `"messages"`) && !strings.Contains(resultStr, `"github.com/napalu/goopt/v2/i18n"`) {
					t.Error("Expected imports to be added when transformations occur")
				}
			}
		})
	}
}

// Test edge cases with composite literals
func TestFormatTransformerCompositeLiteralEdgeCases(t *testing.T) {
	stringMap := map[string]string{
		`"Test"`: "messages.Keys.App.Extracted.Test",
	}

	tests := []struct {
		name          string
		transformMode string
		input         string
		shouldError   bool
	}{
		{
			name:          "empty composite literal",
			transformMode: "with-comments",
			input: `package main
func main() {
	arr := []string{}
}`,
			shouldError: false,
		},
		{
			name:          "composite with only comments",
			transformMode: "with-comments",
			input: `package main
func main() {
	arr := []string{
		// Just a comment
		// Another comment
	}
}`,
			shouldError: false,
		},
		{
			name:          "multiline string in composite",
			transformMode: "with-comments",
			input: `package main
func main() {
	arr := []string{
		` + "`Test`" + `, // i18n-todo: tr.T(messages.Keys.App.Test)
	}
}`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewFormatTransformer(stringMap)
			transformer.SetTransformMode(tt.transformMode)

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
