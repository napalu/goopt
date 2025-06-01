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

// Replacement represents a string replacement to be made
type Replacement struct {
	File        string
	Pos         token.Pos
	End         token.Pos
	Original    string
	Key         string
	Replacement string
	IsComment   bool
}

// CommentReplacer handles adding i18n comments to strings
type CommentReplacer struct {
	tr            i18n.Translator
	trPattern     string
	keepComments  bool
	cleanComments bool
	backupDir     string
	packagePath   string
	replacements  []Replacement
	keyMap        map[string]string // maps strings to keys
}

// NewCommentReplacer creates a new comment replacer
func NewCommentReplacer(tr i18n.Translator, trPattern string, keepComments, cleanComments bool, backupDir, packagePath string) *CommentReplacer {
	if packagePath == "" {
		packagePath = "./messages" // default for backwards compatibility
	}
	return &CommentReplacer{
		tr:            tr,
		trPattern:     trPattern,
		keepComments:  keepComments,
		cleanComments: cleanComments,
		backupDir:     backupDir,
		packagePath:   packagePath,
		keyMap:        make(map[string]string),
	}
}

// SetKeyMap sets the mapping from strings to generated keys
func (cr *CommentReplacer) SetKeyMap(keyMap map[string]string) {
	cr.keyMap = keyMap
}

// ProcessFiles processes the given files and collects replacements
func (cr *CommentReplacer) ProcessFiles(files []string) error {
	for _, file := range files {
		if err := cr.processFile(file); err != nil {
			return fmt.Errorf("error processing %s: %w", file, err)
		}
	}
	return nil
}

// processFile processes a single file
func (cr *CommentReplacer) processFile(filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	// If clean comments mode, just find and remove i18n comments
	if cr.cleanComments {
		cr.findI18nComments(fset, filename, node)
		return nil
	}

	// Find strings to replace
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				cr.processStringLiteral(fset, filename, x)
			}
		}
		return true
	})

	// Find existing i18n-todo comments to potentially remove
	if cr.trPattern != "" && !cr.keepComments {
		cr.findI18nTodoComments(fset, filename, node)
	}

	return nil
}

// processStringLiteral processes a string literal for potential replacement
func (cr *CommentReplacer) processStringLiteral(fset *token.FileSet, filename string, lit *ast.BasicLit) {
	// Get the actual string value (remove quotes)
	value := strings.Trim(lit.Value, "`\"")
	
	// Check if we have a key for this string
	key, ok := cr.keyMap[value]
	if !ok {
		return
	}

	// Create replacement based on mode
	var replacement string
	if cr.trPattern != "" {
		// Replace with translation call
		replacement = cr.createTranslationCall(key, value)
	} else {
		// Add comment
		replacement = fmt.Sprintf("%s // i18n-todo: %s", lit.Value, cr.createTranslationCall(key, value))
	}

	cr.replacements = append(cr.replacements, Replacement{
		File:        filename,
		Pos:         lit.Pos(),
		End:         lit.End(),
		Original:    lit.Value,
		Key:         key,
		Replacement: replacement,
		IsComment:   cr.trPattern == "",
	})
}

// createTranslationCall creates a translation function call
func (cr *CommentReplacer) createTranslationCall(key, value string) string {
	// Convert key to Go constant path
	// app.extracted.user_not_found -> messages.Keys.AppExtracted.UserNotFound
	parts := strings.Split(key, ".")
	var constantPath []string
	
	for _, part := range parts {
		// Convert snake_case to PascalCase
		pascalCase := toPascalCase(part)
		constantPath = append(constantPath, pascalCase)
	}
	
	// Extract package name from path (e.g., "./custompackage" -> "custompackage")
	packageName := cr.packagePath
	if strings.HasPrefix(packageName, "./") {
		packageName = packageName[2:]
	}
	if strings.HasPrefix(packageName, "/") {
		// For absolute paths, use just the last component
		parts := strings.Split(packageName, "/")
		packageName = parts[len(parts)-1]
	}
	
	fullKey := packageName + ".Keys." + strings.Join(constantPath, ".")
	
	pattern := cr.trPattern
	if pattern == "" {
		pattern = "tr.T"
	}
	
	// For direct replacement mode with format strings, we can't add the arguments
	// The developer will need to fix these manually
	if cr.trPattern != "" && strings.Contains(value, "%") {
		// Just return the key without arguments - let the developer fix it
		return fmt.Sprintf("%s(%s)", pattern, fullKey)
	}
	
	// For comment mode, we can indicate it needs arguments
	if cr.trPattern == "" && strings.Contains(value, "%") {
		return fmt.Sprintf("%s(%s, ...)", pattern, fullKey)
	}
	
	return fmt.Sprintf("%s(%s)", pattern, fullKey)
}

// findI18nComments finds all i18n-* comments
func (cr *CommentReplacer) findI18nComments(fset *token.FileSet, filename string, node *ast.File) {
	for _, cg := range node.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "i18n-") {
				cr.replacements = append(cr.replacements, Replacement{
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

// findI18nTodoComments finds i18n-todo comments that should be removed after replacement
func (cr *CommentReplacer) findI18nTodoComments(fset *token.FileSet, filename string, node *ast.File) {
	// Build a map of string positions for strings we're replacing
	stringPositions := make(map[token.Pos]bool)
	for _, r := range cr.replacements {
		if r.File == filename && !r.IsComment {
			stringPositions[r.Pos] = true
		}
	}
	
	// Find all string literals and their positions
	stringLiterals := make(map[token.Pos]*ast.BasicLit)
	ast.Inspect(node, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			stringLiterals[lit.Pos()] = lit
		}
		return true
	})
	
	for _, cg := range node.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "i18n-todo:") {
				// Check if this comment is on the same line as a string we're replacing
				commentPos := fset.Position(c.Pos())
				shouldRemove := false
				
				// Look for string literals on the same line
				for strPos, lit := range stringLiterals {
					strStartPos := fset.Position(strPos)
					strEndPos := fset.Position(lit.End())
					
					// Check if comment is on the same line as string start OR end
					if strStartPos.Line == commentPos.Line || strEndPos.Line == commentPos.Line {
						// Check if this string is being replaced
						if stringPositions[strPos] {
							shouldRemove = true
							break
						}
						
						// Also check if the string value matches one we're replacing
						value := strings.Trim(lit.Value, "`\"")
						if _, hasKey := cr.keyMap[value]; hasKey {
							shouldRemove = true
							break
						}
					}
				}
				
				if shouldRemove {
					cr.replacements = append(cr.replacements, Replacement{
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
}

// ApplyReplacements applies all collected replacements
func (cr *CommentReplacer) ApplyReplacements() error {
	// Group replacements by file
	fileReplacements := make(map[string][]Replacement)
	for _, r := range cr.replacements {
		fileReplacements[r.File] = append(fileReplacements[r.File], r)
	}

	// Create backup directory
	if err := os.MkdirAll(cr.backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Process each file
	for filename, replacements := range fileReplacements {
		if err := cr.applyToFile(filename, replacements); err != nil {
			return fmt.Errorf("failed to update %s: %w", filename, err)
		}
	}

	return nil
}

// applyToFile applies replacements to a single file
func (cr *CommentReplacer) applyToFile(filename string, replacements []Replacement) error {
	// Read file
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(cr.backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create backup
	backupName := fmt.Sprintf("%s_%s.go", 
		strings.TrimSuffix(filepath.Base(filename), ".go"),
		time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(cr.backupDir, backupName)
	if err := os.WriteFile(backupPath, src, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Parse file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	// Sort replacements by position (descending order to maintain positions)
	sortedReplacements := make([]Replacement, len(replacements))
	copy(sortedReplacements, replacements)
	// Sort by position in descending order
	for i := 0; i < len(sortedReplacements); i++ {
		for j := i + 1; j < len(sortedReplacements); j++ {
			if sortedReplacements[i].Pos < sortedReplacements[j].Pos {
				sortedReplacements[i], sortedReplacements[j] = sortedReplacements[j], sortedReplacements[i]
			}
		}
	}

	// Apply replacements using byte offsets
	content := src
	for _, r := range sortedReplacements {
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
		// If replacement is empty, we're removing (e.g., cleaning comments)
		
		newContent = append(newContent, content[endOffset:]...)
		content = newContent
	}
	
	// Check if we need to add imports
	if cr.trPattern != "" && len(replacements) > 0 && !cr.cleanComments {
		// Add messages import if not present
		if !hasImport(node, "messages") {
			content = []byte(addImportToContent(string(content), cr.getMessagePackagePath()))
		}
	}

	// Write back
	if err := os.WriteFile(filename, content, 0644); err != nil {
		return err
	}

	// Format the file
	if err := formatFile(filename); err != nil {
		// Non-fatal error
		fmt.Printf("Warning: failed to format %s: %v\n", filename, err)
	}

	return nil
}

// GetReplacements returns all collected replacements
func (cr *CommentReplacer) GetReplacements() []Replacement {
	return cr.replacements
}

// Helper functions

func toPascalCase(s string) string {
	// Convert snake_case to PascalCase
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

func hasImport(node *ast.File, pkg string) bool {
	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasSuffix(path, "/"+pkg) || path == pkg {
			return true
		}
	}
	return false
}

func addImportToContent(content, importPath string) string {
	// Parse the content to get an AST
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		// Fallback to string manipulation if parsing fails
		return addImportToContentFallback(content, importPath)
	}
	
	// Check if import already exists
	if hasImport(node, importPath) {
		return content
	}
	
	// Create new import spec
	newImport := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("%q", importPath),
		},
	}
	
	// Find or create import declaration
	var importDecl *ast.GenDecl
	for _, decl := range node.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			importDecl = genDecl
			break
		}
	}
	
	if importDecl == nil {
		// Create new import declaration
		importDecl = &ast.GenDecl{
			Tok: token.IMPORT,
			Lparen: token.Pos(1), // This will be adjusted by go/format
			Specs: []ast.Spec{newImport},
			Rparen: token.Pos(1), // This will be adjusted by go/format
		}
		
		// Insert after package declaration
		// Find the position after package declaration
		insertPos := 1 // after package decl
		for i, decl := range node.Decls {
			if _, ok := decl.(*ast.GenDecl); ok {
				// Skip other GenDecls that might be before functions
				continue
			}
			insertPos = i
			break
		}
		
		newDecls := make([]ast.Decl, 0, len(node.Decls)+1)
		newDecls = append(newDecls, node.Decls[:insertPos]...)
		newDecls = append(newDecls, importDecl)
		newDecls = append(newDecls, node.Decls[insertPos:]...)
		node.Decls = newDecls
	} else {
		// Add to existing import block
		importDecl.Specs = append(importDecl.Specs, newImport)
	}
	
	// Format the AST back to source code
	var buf strings.Builder
	if err := format.Node(&buf, fset, node); err != nil {
		// Fallback if formatting fails
		return addImportToContentFallback(content, importPath)
	}
	
	return buf.String()
}

// addImportToContentFallback is the fallback string manipulation method
func addImportToContentFallback(content, importPath string) string {
	importLine := fmt.Sprintf("\t\"%s\"\n", importPath)
	
	// Find import block
	importStart := strings.Index(content, "import (")
	if importStart >= 0 {
		importEnd := strings.Index(content[importStart:], ")")
		if importEnd >= 0 {
			// Insert before closing paren
			insertPos := importStart + importEnd
			return content[:insertPos] + importLine + content[insertPos:]
		}
	}
	
	// No import block found, add one
	packageEnd := strings.Index(content, "\n\n")
	if packageEnd >= 0 {
		return content[:packageEnd] + "\n\nimport (\n" + importLine + ")\n" + content[packageEnd:]
	}
	
	// Last resort: find end of package line
	packageLineEnd := strings.Index(content, "\n")
	if packageLineEnd >= 0 {
		return content[:packageLineEnd] + "\n\nimport (\n" + importLine + ")\n" + content[packageLineEnd+1:]
	}
	
	return content
}

func (cr *CommentReplacer) getMessagePackagePath() string {
	return cr.packagePath
}

func formatFile(filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	formatted, err := format.Source(src)
	if err != nil {
		return err
	}

	return os.WriteFile(filename, formatted, 0644)
}