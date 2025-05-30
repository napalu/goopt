package ast

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/napalu/goopt/v2/i18n"
)

// TransformationReplacer handles replacing strings with translation calls using AST transformation
type TransformationReplacer struct {
	tr                 i18n.Translator
	trPattern          string
	keepComments       bool
	cleanComments      bool
	backupDir          string
	keyMap             map[string]string // maps strings to keys
	formatTransformer  *FormatTransformer
	simpleReplacements []Replacement // for non-format strings
	filesToProcess     []string      // files that need processing
}

// NewTransformationReplacer creates a new string replacer with AST-based format transformation
func NewTransformationReplacer(tr i18n.Translator, trPattern string, keepComments, cleanComments bool, backupDir string) *TransformationReplacer {
	return &TransformationReplacer{
		tr:            tr,
		trPattern:     trPattern,
		keepComments:  keepComments,
		cleanComments: cleanComments,
		backupDir:     backupDir,
		keyMap:        make(map[string]string),
	}
}

// SetKeyMap sets the mapping from strings to generated keys
func (sr *TransformationReplacer) SetKeyMap(keyMap map[string]string) {
	sr.keyMap = keyMap

	// Create a map for the format transformer that maps quoted strings to keys
	quotedKeyMap := make(map[string]string)
	for str, key := range keyMap {
		// Add quotes to match AST BasicLit format
		quotedStr := fmt.Sprintf("%q", str)

		// Convert key format for AST usage
		// app.extracted.hello_world -> messages.Keys.App.Extracted.HelloWorld
		quotedKeyMap[quotedStr] = sr.convertKeyToASTFormat(key)
	}

	sr.formatTransformer = NewFormatTransformer(quotedKeyMap)
}

// ProcessFiles processes the given files
func (sr *TransformationReplacer) ProcessFiles(files []string) error {
	sr.filesToProcess = files

	for _, file := range files {
		if err := sr.processFile(file); err != nil {
			return fmt.Errorf("error processing %s: %w", file, err)
		}
	}
	return nil
}

// processFile processes a single file
func (sr *TransformationReplacer) processFile(filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// If we're using tr pattern (direct replacement mode), use format transformer
	if sr.trPattern != "" {
		// This will be handled in ApplyReplacements
		return nil
	}

	// For comment mode or clean mode, use the simple AST walker
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	// If clean comments mode, just find and remove i18n comments
	if sr.cleanComments {
		sr.findI18nComments(fset, filename, node)
		return nil
	}

	// Find strings to add comments to
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				sr.processStringLiteralForComment(fset, filename, x)
			}
		}
		return true
	})

	return nil
}

// processStringLiteralForComment processes a string literal for comment addition
func (sr *TransformationReplacer) processStringLiteralForComment(fset *token.FileSet, filename string, lit *ast.BasicLit) {
	// Skip if this is inside a format function (will be handled by format transformer)
	if sr.isInFormatFunction(lit) {
		return
	}

	// Get the actual string value (remove quotes)
	value := strings.Trim(lit.Value, "`\"")

	// Check if we have a key for this string
	key, ok := sr.keyMap[value]
	if !ok {
		return
	}

	// Create comment
	replacement := fmt.Sprintf("%s // i18n-todo: %s", lit.Value, sr.createTranslationCall(key, value))

	sr.simpleReplacements = append(sr.simpleReplacements, Replacement{
		File:        filename,
		Pos:         lit.Pos(),
		End:         lit.End(),
		Original:    lit.Value,
		Key:         key,
		Replacement: replacement,
		IsComment:   true,
	})
}

// isInFormatFunction checks if a string literal is inside a format function
func (sr *TransformationReplacer) isInFormatFunction(lit *ast.BasicLit) bool {
	// This is a simplified check - in a real implementation we'd walk up the AST
	// For now, we'll handle all strings
	return false
}

// ApplyReplacements applies all collected replacements
func (sr *TransformationReplacer) ApplyReplacements() error {
	// Create backup directory
	if err := os.MkdirAll(sr.backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	if sr.trPattern != "" {
		// Use format transformer for direct replacement mode
		return sr.applyFormatTransformations()
	}

	// Use simple replacements for comment mode
	return sr.applySimpleReplacements()
}

// applyFormatTransformations uses the format transformer for AST-based transformations
func (sr *TransformationReplacer) applyFormatTransformations() error {
	// Process each file that needs transformation
	processedFiles := make(map[string]bool)

	for _, file := range sr.getFilesToProcess() {
		if processedFiles[file] {
			continue
		}
		processedFiles[file] = true

		// Read file
		src, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		// Create backup
		if err := sr.createBackup(file, src); err != nil {
			return err
		}

		// Transform using format transformer
		transformed, err := sr.formatTransformer.TransformFile(file, src)
		if err != nil {
			return fmt.Errorf("failed to transform %s: %w", file, err)
		}

		// Format the result
		formatted, err := format.Source(transformed)
		if err != nil {
			// If formatting fails, use the unformatted version
			formatted = transformed
		}

		// Write back
		if err := os.WriteFile(file, formatted, 0644); err != nil {
			return err
		}
	}

	return nil
}

// applySimpleReplacements applies simple text replacements (for comment mode)
func (sr *TransformationReplacer) applySimpleReplacements() error {
	// Group replacements by file
	fileReplacements := make(map[string][]Replacement)
	for _, r := range sr.simpleReplacements {
		fileReplacements[r.File] = append(fileReplacements[r.File], r)
	}

	// Process each file
	for filename, replacements := range fileReplacements {
		if err := sr.applySimpleToFile(filename, replacements); err != nil {
			return fmt.Errorf("failed to update %s: %w", filename, err)
		}
	}

	return nil
}

// applySimpleToFile applies simple replacements to a single file
func (sr *TransformationReplacer) applySimpleToFile(filename string, replacements []Replacement) error {
	// Read file
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Create backup
	if err := sr.createBackup(filename, src); err != nil {
		return err
	}

	// Parse file
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	// Sort replacements by position (descending order)
	for i := 0; i < len(replacements); i++ {
		for j := i + 1; j < len(replacements); j++ {
			if replacements[i].Pos < replacements[j].Pos {
				replacements[i], replacements[j] = replacements[j], replacements[i]
			}
		}
	}

	// Apply replacements using byte offsets
	content := src
	for _, r := range replacements {
		// Get byte offsets from token positions
		startOffset := fset.Position(r.Pos).Offset
		endOffset := fset.Position(r.End).Offset

		// Validate offsets
		if startOffset < 0 || endOffset > len(content) || startOffset >= endOffset {
			continue
		}

		// Build the new content
		var newContent []byte
		newContent = append(newContent, content[:startOffset]...)

		if r.Replacement != "" {
			newContent = append(newContent, []byte(r.Replacement)...)
		}

		newContent = append(newContent, content[endOffset:]...)
		content = newContent
	}

	// Write back
	if err := os.WriteFile(filename, content, 0644); err != nil {
		return err
	}

	// Try to format the file
	if formatted, err := format.Source(content); err == nil {
		os.WriteFile(filename, formatted, 0644)
	}

	return nil
}

// getFilesToProcess returns all unique files that need processing
func (sr *TransformationReplacer) getFilesToProcess() []string {
	if sr.trPattern != "" {
		// For AST transformation mode, use the files we were given
		return sr.filesToProcess
	}

	// For comment mode, use files from simple replacements
	fileMap := make(map[string]bool)

	// Collect from simple replacements
	for _, r := range sr.simpleReplacements {
		fileMap[r.File] = true
	}

	files := make([]string, 0, len(fileMap))
	for file := range fileMap {
		files = append(files, file)
	}

	return files
}

// createBackup creates a backup of the file
func (sr *TransformationReplacer) createBackup(filename string, content []byte) error {
	backupName := fmt.Sprintf("%s_%s.go",
		strings.TrimSuffix(filepath.Base(filename), ".go"),
		time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(sr.backupDir, backupName)
	return os.WriteFile(backupPath, content, 0644)
}

// createTranslationCall creates a translation function call string
func (sr *TransformationReplacer) createTranslationCall(key, value string) string {
	// Convert key to Go constant path
	astKey := sr.convertKeyToASTFormat(key)

	pattern := sr.trPattern
	if pattern == "" {
		pattern = "tr.T"
	}

	// For comment mode, indicate format strings need arguments
	if sr.trPattern == "" && strings.Contains(value, "%") {
		return fmt.Sprintf("%s(%s, ...)", pattern, astKey)
	}

	return fmt.Sprintf("%s(%s)", pattern, astKey)
}

// convertKeyToASTFormat converts a key like "app.extracted.hello_world" to "messages.Keys.App.Extracted.HelloWorld"
func (sr *TransformationReplacer) convertKeyToASTFormat(key string) string {
	parts := strings.Split(key, ".")
	var astParts []string

	for _, part := range parts {
		// Convert snake_case to PascalCase
		pascalCase := toPascalCaseV2(part)
		astParts = append(astParts, pascalCase)
	}

	return "messages.Keys." + strings.Join(astParts, ".")
}

// findI18nComments finds all i18n-* comments for cleaning
func (sr *TransformationReplacer) findI18nComments(fset *token.FileSet, filename string, node *ast.File) {
	for _, cg := range node.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "i18n-") {
				sr.simpleReplacements = append(sr.simpleReplacements, Replacement{
					File:        filename,
					Pos:         c.Pos(),
					End:         c.End(),
					Original:    c.Text,
					Replacement: "", // Empty means remove
					IsComment:   true,
				})
			}
		}
	}
}

// GetReplacements returns all collected replacements (for reporting)
func (sr *TransformationReplacer) GetReplacements() []Replacement {
	return sr.simpleReplacements
}

// toPascalCaseV2 converts snake_case to PascalCase
func toPascalCaseV2(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
