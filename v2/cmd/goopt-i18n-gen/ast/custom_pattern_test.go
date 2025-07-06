package ast

import (
	"strings"
	"testing"
)

func TestCustomTranslatorPatterns(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    string
		expected string
		desc     string
	}{
		{
			name:    "function_pattern_TR",
			pattern: "TR().T",
			input: `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}`,
			expected: `package main

import (
	"fmt"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var TR = func() i18n.Translator {
	panic("TODO: Implement TR() - return your i18n.Translator instance")
}

func main() {
	fmt.Println(TR().T(messages.Keys.App.Extracted.Hello))

}`,
			desc: "Should create TR function and use TR().T pattern",
		},
		{
			name:    "function_pattern_lowercase_t",
			pattern: "TR().t",
			input: `package main

func main() {
	msg := "Hello"
}`,
			expected: `package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var TR = func() i18n.Translator {
	panic("TODO: Implement TR() - return your i18n.Translator instance")
}

func main() {
	msg := TR().t(messages.Keys.App.Extracted.Hello)

}`,
			desc: "Should preserve lowercase t in pattern",
		},
		{
			name:    "default_pattern",
			pattern: "tr.T",
			input: `package main

func main() {
	msg := "Hello"
}`,
			expected: `package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var tr i18n.Translator

func main() {
	msg := tr.T(messages.Keys.App.Extracted.Hello)

}`,
			desc: "Default pattern should create var tr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create string map
			stringMap := map[string]string{
				`"Hello"`: "messages.Keys.App.Extracted.Hello",
			}

			// Create transformer with custom pattern
			ft := NewFormatTransformer(stringMap)
			ft.SetTransformMode("all")
			ft.SetTranslatorPattern(tt.pattern)
			ft.SetMessagePackagePath("github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages")

			result, err := ft.TransformFile("test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			// Normalize whitespace
			expected := strings.TrimSpace(tt.expected)
			actual := strings.TrimSpace(string(result))

			if actual != expected {
				t.Errorf("Test %s failed: %s\nExpected:\n%s\n\nGot:\n%s",
					tt.name, tt.desc, expected, actual)
			}
		})
	}
}
