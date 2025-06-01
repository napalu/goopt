package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/napalu/goopt/v2/i18n"
)

// TestFindI18nTodoCommentsPositionBased verifies that comments are removed based on position
func TestFindI18nTodoCommentsPositionBased(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		keyMap          map[string]string
		expectedRemoved []string // comments that should be removed
		expectedKept    []string // comments that should be kept
	}{
		{
			name: "remove only comments on same line as replaced strings",
			code: `package main
func test() {
	msg1 := "Hello world" // i18n-todo: tr.T(messages.Keys.App.HelloWorld)
	msg2 := "Goodbye" // i18n-todo: tr.T(messages.Keys.App.Goodbye)
	msg3 := "Not replaced" // i18n-todo: tr.T(messages.Keys.App.NotReplaced)
}`,
			keyMap: map[string]string{
				"Hello world": "app.hello_world",
				"Goodbye":     "app.goodbye",
			},
			expectedRemoved: []string{
				"// i18n-todo: tr.T(messages.Keys.App.HelloWorld)",
				"// i18n-todo: tr.T(messages.Keys.App.Goodbye)",
			},
			expectedKept: []string{
				"// i18n-todo: tr.T(messages.Keys.App.NotReplaced)",
			},
		},
		{
			name: "handle multiline strings correctly",
			code: `package main
func test() {
	query := ` + "`" + `SELECT * 
		FROM users
		WHERE active = true` + "`" + ` // i18n-todo: tr.T(messages.Keys.App.Query)
	other := "other string" // i18n-todo: tr.T(messages.Keys.App.Other)
}`,
			keyMap: map[string]string{
				"SELECT * \n\t\tFROM users\n\t\tWHERE active = true": "app.query",
			},
			expectedRemoved: []string{
				"// i18n-todo: tr.T(messages.Keys.App.Query)",
			},
			expectedKept: []string{
				"// i18n-todo: tr.T(messages.Keys.App.Other)",
			},
		},
		{
			name: "don't remove comments on different lines",
			code: `package main
func test() {
	// i18n-todo: tr.T(messages.Keys.App.Something)
	msg := "Hello"
	
	value := "World"
	// i18n-todo: tr.T(messages.Keys.App.World)
}`,
			keyMap: map[string]string{
				"Hello": "app.hello",
			},
			expectedRemoved: []string{},
			expectedKept: []string{
				"// i18n-todo: tr.T(messages.Keys.App.Something)",
				"// i18n-todo: tr.T(messages.Keys.App.World)",
			},
		},
		{
			name: "handle multiple strings on same line",
			code: `package main
func test() {
	fmt.Printf("Format: %s", "value") // i18n-todo: tr.T(messages.Keys.App.Format)
}`,
			keyMap: map[string]string{
				"Format: %s": "app.format_s",
			},
			expectedRemoved: []string{
				"// i18n-todo: tr.T(messages.Keys.App.Format)",
			},
			expectedKept: []string{},
		},
		{
			name: "comments between string and line end",
			code: `package main
func test() {
	msg := "Test" /* inline */ // i18n-todo: tr.T(messages.Keys.App.Test)
	other := "Other" // i18n-todo: tr.T(messages.Keys.App.Other)
	third := "Third" // not i18n comment
}`,
			keyMap: map[string]string{
				"Test": "app.test",
			},
			expectedRemoved: []string{
				"// i18n-todo: tr.T(messages.Keys.App.Test)",
			},
			expectedKept: []string{
				"// i18n-todo: tr.T(messages.Keys.App.Other)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := i18n.NewEmptyBundle()
			cr := NewCommentReplacer(bundle, "tr.T", false, false, ".backup", "./messages")
			cr.SetKeyMap(tt.keyMap)
			
			// Parse the code
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}
			
			// First, process string literals to build replacements
			ast.Inspect(node, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					cr.processStringLiteral(fset, "test.go", lit)
				}
				return true
			})
			
			// Then find i18n-todo comments
			cr.findI18nTodoComments(fset, "test.go", node)
			
			// Check which comments were marked for removal
			removedComments := []string{}
			keptComments := []string{}
			
			for _, cg := range node.Comments {
				for _, c := range cg.List {
					if strings.Contains(c.Text, "i18n-todo:") {
						found := false
						for _, r := range cr.replacements {
							if r.IsComment && r.Original == c.Text && r.Replacement == "" {
								removedComments = append(removedComments, c.Text)
								found = true
								break
							}
						}
						if !found {
							keptComments = append(keptComments, c.Text)
						}
					}
				}
			}
			
			// Verify expected removals
			for _, expected := range tt.expectedRemoved {
				found := false
				for _, removed := range removedComments {
					if removed == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected comment to be removed but it wasn't: %s", expected)
				}
			}
			
			// Verify expected kept comments
			for _, expected := range tt.expectedKept {
				found := false
				for _, kept := range keptComments {
					if kept == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected comment to be kept but it was removed: %s", expected)
				}
			}
			
			// Also verify no unexpected removals
			if len(removedComments) != len(tt.expectedRemoved) {
				t.Errorf("Expected %d comments to be removed, but %d were removed", 
					len(tt.expectedRemoved), len(removedComments))
			}
		})
	}
}

// TestAddImportToContentAST verifies AST-based import addition
func TestAddImportToContentAST(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		importPath string
		verify     func(t *testing.T, result string)
	}{
		{
			name: "add to existing import block",
			content: `package main

import (
	"fmt"
)

func main() {
	fmt.Println("test")
}`,
			importPath: "./messages",
			verify: func(t *testing.T, result string) {
				// Should contain both imports
				if !strings.Contains(result, `"fmt"`) {
					t.Error("Lost existing fmt import")
				}
				if !strings.Contains(result, `"./messages"`) {
					t.Error("Failed to add messages import")
				}
				// Should be in import block
				if !strings.Contains(result, "import (") {
					t.Error("Import block not preserved")
				}
			},
		},
		{
			name: "create new import block",
			content: `package main

func main() {
	println("test")
}`,
			importPath: "./messages",
			verify: func(t *testing.T, result string) {
				if !strings.Contains(result, `import (`) {
					t.Error("Failed to create import block")
				}
				if !strings.Contains(result, `"./messages"`) {
					t.Error("Failed to add messages import")
				}
			},
		},
		{
			name: "don't add duplicate import",
			content: `package main

import (
	"fmt"
	"./messages"
)

func main() {
	fmt.Println("test")
}`,
			importPath: "./messages",
			verify: func(t *testing.T, result string) {
				// Count occurrences of messages import
				count := strings.Count(result, `"./messages"`)
				if count != 1 {
					t.Errorf("Expected exactly 1 messages import, found %d", count)
				}
			},
		},
		{
			name: "handle single import statement",
			content: `package main

import "fmt"

func main() {
	fmt.Println("test")
}`,
			importPath: "./messages",
			verify: func(t *testing.T, result string) {
				// Should convert to import block
				if !strings.Contains(result, "import (") {
					t.Error("Failed to convert single import to block")
				}
				if !strings.Contains(result, `"fmt"`) {
					t.Error("Lost existing fmt import")
				}
				if !strings.Contains(result, `"./messages"`) {
					t.Error("Failed to add messages import")
				}
			},
		},
		{
			name: "handle import with alias",
			content: `package main

import (
	"fmt"
	msg "example.com/old-messages"
)

func main() {
	fmt.Println("test")
}`,
			importPath: "./messages",
			verify: func(t *testing.T, result string) {
				if !strings.Contains(result, `msg "example.com/old-messages"`) {
					t.Error("Lost aliased import")
				}
				if !strings.Contains(result, `"./messages"`) {
					t.Error("Failed to add messages import")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addImportToContent(tt.content, tt.importPath)
			
			// Verify it's valid Go code
			_, err := parser.ParseFile(token.NewFileSet(), "", result, parser.ParseComments)
			if err != nil {
				t.Errorf("Result is not valid Go code: %v", err)
			}
			
			// Run specific verifications
			tt.verify(t, result)
		})
	}
}

// TestAddImportToContentEdgeCases tests edge cases and fallback behavior
func TestAddImportToContentEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		importPath string
		shouldUseAST bool
	}{
		{
			name:       "malformed go code uses fallback",
			content:    `this is not valid go code`,
			importPath: "./messages",
			shouldUseAST: false,
		},
		{
			name: "complex but valid code uses AST",
			content: `package main

import (
	"fmt"
	"log"
	
	"github.com/example/pkg"
)

func main() {}`,
			importPath: "./messages",
			shouldUseAST: true,
		},
		{
			name: "code with syntax error uses fallback",
			content: `package main

import (
	"fmt"

func main() {}`,
			importPath: "./messages",
			shouldUseAST: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := addImportToContent(tt.content, tt.importPath)
			
			// Should return something
			if result == "" {
				t.Error("Expected non-empty result")
			}
			
			// If using AST, result should be valid Go
			if tt.shouldUseAST {
				_, err := parser.ParseFile(token.NewFileSet(), "", result, parser.ParseComments)
				if err != nil {
					t.Errorf("Expected valid Go code when using AST, got error: %v", err)
				}
			}
		})
	}
}

// TestCommentReplacerFullIntegration tests the full workflow
func TestCommentReplacerFullIntegration(t *testing.T) {
	testCode := `package main

import "fmt"

func main() {
	// Existing i18n-todo comments
	msg1 := "Welcome" // i18n-todo: tr.T(messages.Keys.App.Welcome)
	msg2 := "Goodbye" // i18n-todo: tr.T(messages.Keys.App.Goodbye) 
	
	// String without comment
	msg3 := "Hello world"
	
	// String that won't be replaced
	msg4 := "Not in keymap" // i18n-todo: tr.T(messages.Keys.App.NotInKeymap)
	
	fmt.Println(msg1, msg2, msg3, msg4)
}`

	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	bundle := i18n.NewEmptyBundle()
	cr := NewCommentReplacer(bundle, "tr.T", false, false, filepath.Join(tmpDir, "backup"), "./messages")
	
	// Set up key map
	keyMap := map[string]string{
		"Welcome":     "app.welcome",
		"Goodbye":     "app.goodbye",
		"Hello world": "app.hello_world",
	}
	cr.SetKeyMap(keyMap)
	
	// Process file
	if err := cr.ProcessFiles([]string{testFile}); err != nil {
		t.Fatalf("Failed to process files: %v", err)
	}
	
	// Apply replacements
	if err := cr.ApplyReplacements(); err != nil {
		t.Fatalf("Failed to apply replacements: %v", err)
	}
	
	// Read result
	result, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read result: %v", err)
	}
	
	resultStr := string(result)
	
	// Verify strings were replaced
	if !strings.Contains(resultStr, `tr.T(messages.Keys.App.Welcome)`) {
		t.Error("Welcome string should be replaced")
	}
	if !strings.Contains(resultStr, `tr.T(messages.Keys.App.Goodbye)`) {
		t.Error("Goodbye string should be replaced")
	}
	if !strings.Contains(resultStr, `tr.T(messages.Keys.App.HelloWorld)`) {
		t.Error("Hello world string should be replaced")
	}
	
	// Verify i18n-todo comments were removed for replaced strings
	if strings.Contains(resultStr, `// i18n-todo: tr.T(messages.Keys.App.Welcome)`) {
		t.Error("i18n-todo comment for Welcome should be removed")
	}
	if strings.Contains(resultStr, `// i18n-todo: tr.T(messages.Keys.App.Goodbye)`) {
		t.Error("i18n-todo comment for Goodbye should be removed")
	}
	
	// Verify i18n-todo comment for non-replaced string remains
	if !strings.Contains(resultStr, `// i18n-todo: tr.T(messages.Keys.App.NotInKeymap)`) {
		t.Error("i18n-todo comment for non-replaced string should remain")
	}
	
	// Verify import was added
	if !strings.Contains(resultStr, `"./messages"`) {
		t.Error("Messages import should be added")
	}
	
	// Verify it's valid Go code
	_, err = parser.ParseFile(token.NewFileSet(), "", resultStr, parser.ParseComments)
	if err != nil {
		t.Errorf("Result is not valid Go code: %v", err)
	}
}

// BenchmarkAddImportToContent tests performance of AST vs string manipulation
func BenchmarkAddImportToContent(b *testing.B) {
	content := `package main

import (
	"fmt"
	"log"
	"strings"
)

func main() {
	fmt.Println("test")
}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = addImportToContent(content, "./messages")
	}
}

// BenchmarkFindI18nTodoComments tests performance of position-based comment finding
func BenchmarkFindI18nTodoComments(b *testing.B) {
	code := `package main

func test() {
	msg1 := "Hello" // i18n-todo: tr.T(messages.Keys.App.Hello)
	msg2 := "World" // i18n-todo: tr.T(messages.Keys.App.World)
	msg3 := "Test"  // i18n-todo: tr.T(messages.Keys.App.Test)
}`

	bundle := i18n.NewEmptyBundle()
	cr := NewCommentReplacer(bundle, "tr.T", false, false, ".backup", "./messages")
	cr.SetKeyMap(map[string]string{
		"Hello": "app.hello",
		"World": "app.world",
	})

	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, "test.go", code, parser.ParseComments)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cr.replacements = nil // Reset
		cr.findI18nTodoComments(fset, "test.go", node)
	}
}