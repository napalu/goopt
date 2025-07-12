package ast

import (
	"strings"
	"testing"
)

func TestImportAndVarInsertion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		desc     string
	}{
		{
			name: "simple_file_no_imports",
			input: `package main

type Logger struct{}

func main() {
	msg := "Hello"
}`,
			expected: `package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var tr i18n.Translator

type Logger struct{}

func main() {
	msg := tr.T(messages.Keys.App.Extracted.Hello)

}`,
			desc: "Should insert imports and var after package, before type",
		},
		{
			name: "file_with_go_embed",
			input: `package main

//go:embed static/*
var staticFS embed.FS

func main() {
	msg := "Hello"
}`,
			expected: `package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var tr i18n.Translator

//go:embed static/*
var staticFS embed.FS

func main() {
	msg := tr.T(messages.Keys.App.Extracted.Hello)

}`,
			desc: "Should not break //go:embed directive",
		},
		{
			name: "file_with_go_generate",
			input: `package main

//go:generate mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE
type Service interface {
	GetMessage() string
}

func getMessage() string {
	return "Hello"
}`,
			expected: `package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var tr i18n.Translator

//go:generate mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE
type Service interface {
	GetMessage() string
}

func getMessage() string {
	return tr.T(messages.Keys.App.Extracted.Hello)

}`,
			desc: "Should not break //go:generate directive",
		},
		{
			name: "file_with_multiple_go_directives",
			input: `package main

//go:build !windows
// +build !windows

//go:embed templates/*.html
var templates embed.FS

//go:generate go run gen.go

type Config struct {
	Message string
}

func main() {
	cfg := Config{Message: "Hello"}
}`,
			expected: `//go:build !windows
// +build !windows

package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var tr i18n.Translator

//go:embed templates/*.html
var templates embed.FS

//go:generate go run gen.go

type Config struct {
	Message string
}

func main() {
	cfg := Config{Message: tr.T(messages.Keys.App.Extracted.Hello)}
}`,
			desc: "Should handle multiple //go: directives correctly",
		},
		{
			name: "file_with_const_before_var",
			input: `package main

const AppName = "MyApp"

var Version = "1.0.0"

func main() {
	msg := "Hello from MyApp"
}`,
			expected: `package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var tr i18n.Translator

const AppName = "MyApp"

var Version = "1.0.0"

func main() {
	msg := tr.T(messages.Keys.App.Extracted.HelloFromMyApp)

}`,
			desc: "Should handle const declarations before var",
		},
		{
			name: "file_with_existing_imports",
			input: `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello")
}`,
			expected: `package main

import (
	"fmt"
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
	"os"
)

var tr i18n.Translator

func main() {
	fmt.Println(tr.T(messages.Keys.App.Extracted.Hello))

}`,
			desc: "Should add to existing imports",
		},
		{
			name: "file_with_comment_before_package",
			input: `// Package main provides the entry point
package main

//go:embed assets
var assets embed.FS

func main() {
	msg := "Hello"
}`,
			expected: `// Package main provides the entry point
package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var tr i18n.Translator

//go:embed assets
var assets embed.FS

func main() {
	msg := tr.T(messages.Keys.App.Extracted.Hello)

}`,
			desc: "Should handle package comments correctly",
		},
		{
			name: "file_with_build_tags",
			input: `//go:build linux && amd64
// +build linux,amd64

package main

func main() {
	msg := "Hello Linux"
}`,
			expected: `//go:build linux && amd64
// +build linux,amd64

package main

import (
	"github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages"
	"github.com/napalu/goopt/v2/i18n"
)

var tr i18n.Translator

func main() {
	msg := tr.T(messages.Keys.App.Extracted.HelloLinux)

}`,
			desc: "Should handle build tags before package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a simple string map for transformation
			stringMap := make(map[string]string)
			// Extract all quoted strings from input and create keys
			lines := strings.Split(tt.input, "\n")
			for _, line := range lines {
				// Only transform strings that should be transformed
				if !shouldTransformString(line) {
					continue
				}

				if idx := strings.Index(line, `"`); idx >= 0 {
					if endIdx := strings.Index(line[idx+1:], `"`); endIdx >= 0 {
						str := line[idx : idx+endIdx+2]
						// Generate a valid key
						key := generateValidKey("app.extracted.", str)
						stringMap[str] = key
					}
				}
			}

			// Create transformer and transform
			ft := NewFormatTransformer(stringMap)
			ft.SetTransformMode("all")
			ft.SetMessagePackagePath("github.com/napalu/goopt/v2/cmd/goopt-i18n-gen/messages")

			result, err := ft.TransformFile("test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("TransformFile failed: %v", err)
			}

			// Normalize whitespace for comparison
			expected := strings.TrimSpace(tt.expected)
			actual := strings.TrimSpace(string(result))

			if actual != expected {
				t.Errorf("Test %s failed: %s\nExpected:\n%s\n\nGot:\n%s\n\nDiff:",
					tt.name, tt.desc, expected, actual)

				// Show line-by-line diff
				expectedLines := strings.Split(expected, "\n")
				actualLines := strings.Split(actual, "\n")

				maxLines := len(expectedLines)
				if len(actualLines) > maxLines {
					maxLines = len(actualLines)
				}

				for i := 0; i < maxLines; i++ {
					if i < len(expectedLines) && i < len(actualLines) {
						if expectedLines[i] != actualLines[i] {
							t.Errorf("Line %d differs:\n  Expected: %s\n  Got:      %s",
								i+1, expectedLines[i], actualLines[i])
						}
					} else if i >= len(actualLines) {
						t.Errorf("Line %d missing in actual output:\n  Expected: %s",
							i+1, expectedLines[i])
					} else {
						t.Errorf("Line %d extra in actual output:\n  Got: %s",
							i+1, actualLines[i])
					}
				}
			}
		})
	}
}

// TestGoDirectiveDetection tests the detection of //go: directives
func TestGoDirectiveDetection(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		hasDirective bool
	}{
		{
			name: "go_embed",
			input: `//go:embed static/*
var fs embed.FS`,
			hasDirective: true,
		},
		{
			name: "go_generate",
			input: `//go:generate mockgen
type Interface interface{}`,
			hasDirective: true,
		},
		{
			name: "go_build",
			input: `//go:build linux
package main`,
			hasDirective: true,
		},
		{
			name: "regular_comment",
			input: `// This is a regular comment
var x int`,
			hasDirective: false,
		},
		{
			name: "go_comment_with_space",
			input: `// go:embed (with space - not a directive)
var x int`,
			hasDirective: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would test the actual directive detection logic
			// For now, we're just documenting the test cases
			t.Logf("Test case %s: expecting hasDirective=%v", tt.name, tt.hasDirective)
		})
	}
}
