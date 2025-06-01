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

// TestFullIntegration tests all three fixes working together
func TestFullIntegration(t *testing.T) {
	// Test code that exercises all three fixes
	testCode := `package main

import (
	"fmt"
	"log"
)

type Logger struct{}

func (l *Logger) Info() *Logger { return l }
func (l *Logger) Err(error) *Logger { return l }
func (l *Logger) Msg(string) {}
func (l *Logger) Msgf(string, ...interface{}) {}

func main() {
	logger := &Logger{}
	
	// Test 1: Format functions should be skipped
	fmt.Printf("Server starting on port %d", 8080)
	log.Printf("Database connected: %s", dbName)
	
	// Test 2: Chained logging should be detected
	logger.Info().Msg("Application initialized")
	logger.Err(err).Msgf("Failed to process file: %s", filename)
	
	// Test 3: Regular strings should get comments
	status := "Processing request"
	
	// Test 4: Existing i18n-todo comments
	msg1 := "Welcome user" // i18n-todo: tr.T(messages.Keys.App.WelcomeUser)
	msg2 := "Goodbye"      // i18n-todo: tr.T(messages.Keys.App.Goodbye)
	
	fmt.Println(status, msg1, msg2)
}
`

	// Create a temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	
	// Write test file
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test scenario 1: Comment mode (adds i18n-todo comments)
	t.Run("comment mode", func(t *testing.T) {
		// Restore original file
		if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
			t.Fatalf("Failed to restore test file: %v", err)
		}

		bundle := i18n.NewEmptyBundle()
		tr := NewTransformationReplacer(bundle, "", false, false, filepath.Join(tmpDir, "backup"), "./messages")
		
		keyMap := map[string]string{
			"Server starting on port %d":  "app.server_starting_on_port_d",
			"Database connected: %s":      "app.database_connected_s",
			"Application initialized":     "app.application_initialized",
			"Failed to process file: %s":  "app.failed_to_process_file_s",
			"Processing request":          "app.processing_request",
			"Welcome user":                "app.welcome_user",
			"Goodbye":                     "app.goodbye",
		}
		tr.SetKeyMap(keyMap)
		
		// Process file
		if err := tr.ProcessFiles([]string{testFile}); err != nil {
			t.Fatalf("Failed to process files: %v", err)
		}
		
		// Apply replacements
		if err := tr.ApplyReplacements(); err != nil {
			t.Fatalf("Failed to apply replacements: %v", err)
		}
		
		// Read result
		result, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read result: %v", err)
		}
		
		resultStr := string(result)
		
		// Verify format function strings were NOT commented
		if strings.Contains(resultStr, `"Server starting on port %d" // i18n-todo`) {
			t.Error("Format function string should not have i18n-todo comment")
		}
		if strings.Contains(resultStr, `"Database connected: %s" // i18n-todo`) {
			t.Error("Log format string should not have i18n-todo comment")
		}
		
		// Verify chained logging strings were NOT commented
		if strings.Contains(resultStr, `"Application initialized" // i18n-todo`) {
			t.Error("Chained logging string should not have i18n-todo comment")
		}
		if strings.Contains(resultStr, `"Failed to process file: %s" // i18n-todo`) {
			t.Error("Chained logging format string should not have i18n-todo comment")
		}
		
		// Verify regular string WAS commented
		if !strings.Contains(resultStr, `"Processing request" // i18n-todo: tr.T(messages.Keys.App.ProcessingRequest)`) {
			t.Error("Regular string should have i18n-todo comment")
		}
		
		// Verify existing comments remain
		if !strings.Contains(resultStr, `"Welcome user" // i18n-todo: tr.T(messages.Keys.App.WelcomeUser)`) {
			t.Error("Existing i18n-todo comment should remain")
		}
	})
	
	// Test scenario 2: Direct replacement mode
	t.Run("direct replacement mode", func(t *testing.T) {
		// Restore original file
		if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
			t.Fatalf("Failed to restore test file: %v", err)
		}
		
		bundle := i18n.NewEmptyBundle()
		cr := NewCommentReplacer(bundle, "tr.T", false, false, filepath.Join(tmpDir, "backup2"), "./messages")
		
		keyMap := map[string]string{
			"Processing request": "app.processing_request",
			"Welcome user":       "app.welcome_user",
			"Goodbye":           "app.goodbye",
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
		if !strings.Contains(resultStr, `tr.T(messages.Keys.App.ProcessingRequest)`) {
			t.Error("String should be replaced with translation call")
		}
		
		// Verify i18n-todo comments were removed for replaced strings
		if strings.Contains(resultStr, `// i18n-todo: tr.T(messages.Keys.App.WelcomeUser)`) {
			t.Error("i18n-todo comment should be removed for replaced string")
		}
		if strings.Contains(resultStr, `// i18n-todo: tr.T(messages.Keys.App.Goodbye)`) {
			t.Error("i18n-todo comment should be removed for replaced string")
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
	})
}

// TestRegressionScenarios tests specific scenarios that would break with our bugs
func TestRegressionScenarios(t *testing.T) {
	t.Run("bug 1: isInUserFacingFunction always returning false", func(t *testing.T) {
		// This would cause format strings to get i18n comments when they shouldn't
		code := `package main
import "fmt"
func main() {
	fmt.Printf("User %s logged in", username)
}`
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.go")
		if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		bundle := i18n.NewEmptyBundle()
		tr := NewTransformationReplacer(bundle, "", false, false, filepath.Join(tmpDir, "backup"), "./messages")
		tr.SetKeyMap(map[string]string{
			"User %s logged in": "app.user_logged_in",
		})
		
		if err := tr.ProcessFiles([]string{testFile}); err != nil {
			t.Fatalf("Failed to process: %v", err)
		}
		
		if err := tr.ApplyReplacements(); err != nil {
			t.Fatalf("Failed to apply: %v", err)
		}
		
		result, _ := os.ReadFile(testFile)
		resultStr := string(result)
		
		// Should NOT have i18n-todo comment because it's in Printf
		if strings.Contains(resultStr, `// i18n-todo`) {
			t.Error("Bug regression: Format string in Printf got i18n-todo comment")
		}
	})

	t.Run("bug 2: findI18nTodoComments removing all comments", func(t *testing.T) {
		// This would remove i18n-todo comments that shouldn't be removed
		code := `package main
func main() {
	msg1 := "Hello" // i18n-todo: tr.T(messages.Keys.App.Hello)
	msg2 := "World" // i18n-todo: tr.T(messages.Keys.App.World)
}`
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.go")
		if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		bundle := i18n.NewEmptyBundle()
		cr := NewCommentReplacer(bundle, "tr.T", false, false, filepath.Join(tmpDir, "backup"), "./messages")
		// Only replace "Hello", not "World"
		cr.SetKeyMap(map[string]string{
			"Hello": "app.hello",
		})
		
		if err := cr.ProcessFiles([]string{testFile}); err != nil {
			t.Fatalf("Failed to process: %v", err)
		}
		
		if err := cr.ApplyReplacements(); err != nil {
			t.Fatalf("Failed to apply: %v", err)
		}
		
		result, _ := os.ReadFile(testFile)
		resultStr := string(result)
		
		// Should still have the World i18n-todo comment
		if !strings.Contains(resultStr, `// i18n-todo: tr.T(messages.Keys.App.World)`) {
			t.Error("Bug regression: i18n-todo comment removed when string wasn't replaced")
		}
	})

	t.Run("bug 3: addImportToContent using string manipulation", func(t *testing.T) {
		// Test that imports are added correctly even with complex import blocks
		code := `package main

import (
	"fmt"
	_ "embed"
	
	"github.com/pkg/errors"
)

func main() {}
`
		result := addImportToContent(code, "./messages")
		
		// Should preserve the import structure
		if !strings.Contains(result, `_ "embed"`) {
			t.Error("Bug regression: Lost blank import when adding new import")
		}
		
		// Should add the new import
		if !strings.Contains(result, `"./messages"`) {
			t.Error("Bug regression: Failed to add import")
		}
		
		// Should be valid Go code
		_, err := parser.ParseFile(token.NewFileSet(), "", result, parser.ParseComments)
		if err != nil {
			t.Errorf("Bug regression: Result is not valid Go code: %v", err)
		}
	})
}

// TestEdgeCasesAndErrorHandling tests error handling
func TestEdgeCasesAndErrorHandling(t *testing.T) {
	t.Run("nil literal handling", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		tr := &TransformationReplacer{
			tr:          bundle,
			parentStack: []ast.Node{},
		}
		
		// Should not panic
		result := tr.isInUserFacingFunction(nil)
		if result {
			t.Error("Expected false for nil literal")
		}
	})
	
	t.Run("empty parent stack", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		tr := &TransformationReplacer{
			tr:          bundle,
			parentStack: []ast.Node{},
		}
		
		lit := &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"test"`,
		}
		
		// Should not panic with empty parent stack
		result := tr.isInUserFacingFunction(lit)
		if result {
			t.Error("Expected false for empty parent stack")
		}
	})
	
	t.Run("nil AST walking", func(t *testing.T) {
		bundle := i18n.NewEmptyBundle()
		tr := &TransformationReplacer{
			tr: bundle,
		}
		
		// Should handle nil gracefully
		called := false
		tr.walkASTWithParents(nil, func(n ast.Node, parents []ast.Node) bool {
			called = true
			return true
		})
		
		if called {
			t.Error("Visit function should not be called for nil node")
		}
	})
	
	t.Run("malformed Go code", func(t *testing.T) {
		malformed := `package main
		this is not valid go`
		
		result := addImportToContent(malformed, "./messages")
		
		// Should still return something (fallback)
		if result == "" {
			t.Error("Should return non-empty result even for malformed code")
		}
		
		// Should attempt to add the import
		if !strings.Contains(result, "./messages") {
			t.Error("Should attempt to add import even to malformed code")
		}
	})
}

// Benchmark tests to ensure performance
func BenchmarkIsInFormatFunctionIntegration(b *testing.B) {
	bundle := i18n.NewEmptyBundle()
	tr := &TransformationReplacer{
		tr: bundle,
		// Simulate a deep parent stack
		parentStack: []ast.Node{
			&ast.File{},
			&ast.FuncDecl{},
			&ast.BlockStmt{},
			&ast.ExprStmt{},
			&ast.CallExpr{},
			&ast.CallExpr{},
			&ast.SelectorExpr{},
		},
	}
	
	lit := &ast.BasicLit{
		Kind:  token.STRING,
		Value: `"test string"`,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tr.isInUserFacingFunction(lit)
	}
}

func BenchmarkAddImportToContentIntegration(b *testing.B) {
	content := `package main

import (
	"fmt"
	"log"
	"strings"
)

func main() {
	fmt.Println("test")
}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = addImportToContent(content, "./messages")
	}
}