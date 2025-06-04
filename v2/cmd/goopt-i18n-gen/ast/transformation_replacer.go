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

// TransformationConfig holds configuration for the TransformationReplacer
type TransformationConfig struct {
	Translator           i18n.Translator
	TrPattern            string
	KeepComments         bool
	CleanComments        bool
	IsUpdateMode         bool
	TransformMode        string   // "user-facing", "with-comments", "all-marked", "all"
	BackupDir            string
	PackagePath          string
	UserFacingRegex      []string // regex patterns to identify user-facing functions
	FormatFunctionRegex  []string // regex patterns with format arg index (pattern:index)
}

// TransformationReplacer handles replacing strings with translation calls using AST transformation
type TransformationReplacer struct {
	config             *TransformationConfig
	keyMap             map[string]string // maps strings to keys
	formatTransformer  *FormatTransformer
	simpleReplacements []Replacement // for non-format strings
	filesToProcess     []string      // files that need processing
	parentStack        []ast.Node    // track parent nodes during AST walking
}

// NewTransformationReplacer creates a new string replacer with AST-based format transformation
func NewTransformationReplacer(config *TransformationConfig) *TransformationReplacer {
	if config.PackagePath == "" {
		config.PackagePath = "messages" // default (will be resolved to full module path)
	}
	return &TransformationReplacer{
		config: config,
		keyMap: make(map[string]string),
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
		// app.extracted.hello_world -> packageName.Keys.App.Extracted.HelloWorld
		quotedKeyMap[quotedStr] = sr.convertKeyToASTFormat(key)
	}

	sr.formatTransformer = NewFormatTransformer(quotedKeyMap)
	sr.formatTransformer.SetMessagePackagePath(sr.config.PackagePath)
	sr.formatTransformer.SetTransformMode(sr.config.TransformMode)
	
	// Set user-facing regex patterns
	if len(sr.config.UserFacingRegex) > 0 {
		if err := sr.formatTransformer.SetUserFacingRegexes(sr.config.UserFacingRegex); err != nil {
			// Log error but continue - we'll fall back to default behavior
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}
	
	// Set format function patterns
	if len(sr.config.FormatFunctionRegex) > 0 {
		if err := sr.formatTransformer.SetFormatFunctionPatterns(sr.config.FormatFunctionRegex); err != nil {
			// Log error but continue
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}
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

	// If we're in update mode (direct replacement), use format transformer
	if sr.config.IsUpdateMode {
		// The actual transformation will be handled in ApplyReplacements
		// Comment removal will happen after transformation if needed
		return nil
	}

	// For comment mode or clean mode, use the simple AST walker
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	// If clean comments mode, just find and remove i18n comments
	if sr.config.CleanComments {
		sr.findI18nComments(fset, filename, node)
		return nil
	}

	// Find strings to add comments to using custom walker that tracks parents
	sr.walkASTWithParents(node, func(n ast.Node, parents []ast.Node) bool {
		switch x := n.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				sr.parentStack = parents
				sr.processStringLiteralForComment(fset, filename, x)
			}
		}
		return true
	})

	return nil
}

// processStringLiteralForComment processes a string literal for comment addition
func (sr *TransformationReplacer) processStringLiteralForComment(fset *token.FileSet, filename string, lit *ast.BasicLit) {
	// In comment mode, we want to add comments to ALL strings that have keys
	// Don't skip user-facing functions - we're just adding comments, not transforming

	// Get the actual string value (remove quotes)
	value := strings.Trim(lit.Value, "`\"")

	// Check if we have a key for this string
	key, ok := sr.keyMap[value]
	if !ok {
		return
	}

	// Skip multi-line raw string literals (backtick strings with newlines)
	// These can break syntax when comments are added inline
	if strings.HasPrefix(lit.Value, "`") && strings.Contains(lit.Value, "\n") {
		return
	}

	// Create more helpful comment based on context
	comment := sr.createCommentForContext(key, value, lit)
	replacement := fmt.Sprintf("%s /* i18n-todo: %s */", lit.Value, comment)

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

// isInUserFacingFunction checks if a string literal is inside a function that displays user-facing text
// This includes format functions (Printf, Errorf) and regular display functions (Print, Log, Msg)
func (sr *TransformationReplacer) isInUserFacingFunction(lit *ast.BasicLit) bool {
	// Walk up the parent stack to find if we're in a user-facing function call
	for i := len(sr.parentStack) - 1; i >= 0; i-- {
		parent := sr.parentStack[i]

		if callExpr, ok := parent.(*ast.CallExpr); ok {
			// Check if this is a user-facing function
			funcName := sr.getFunctionName(callExpr)

			// List of known functions that display user-facing text
			userFacingFunctions := map[string]bool{
				"fmt.Printf":    true,
				"fmt.Sprintf":   true,
				"fmt.Fprintf":   true,
				"fmt.Errorf":    true,
				"log.Printf":    true,
				"log.Fatalf":    true,
				"log.Panicf":    true,
				"errors.New":    true,
				"errors.Errorf": true,
				"errors.Wrapf":  true,
			}

			// Check for method chain patterns (e.g., logger.Info().Msg())
			if sr.isChainedLoggingCall(callExpr, lit) {
				return true
			}

			if userFacingFunctions[funcName] {
				// Check if our literal is the format string (first or second argument)
				for idx, arg := range callExpr.Args {
					if arg == lit {
						// For Fprintf and Wrapf, format string is second argument
						if (strings.HasSuffix(funcName, ".Fprintf") || strings.HasSuffix(funcName, ".Wrapf")) && idx == 1 {
							return true
						}
						// For others, format string is first argument
						if !strings.HasSuffix(funcName, ".Fprintf") && !strings.HasSuffix(funcName, ".Wrapf") && idx == 0 {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// isChainedLoggingCall checks for chained logging patterns like logger.Info().Msg("text")
func (sr *TransformationReplacer) isChainedLoggingCall(call *ast.CallExpr, lit *ast.BasicLit) bool {
	// Check if the function name ends with logging methods
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName := sel.Sel.Name

		// Check for message methods that take a string
		// These are methods that commonly take user-facing strings
		messageMethods := map[string]bool{
			"Msg":    true,
			"Msgf":   true,
			"Send":   true, // for zerolog
			"Str":    true, // for field methods like .Str("key", "value") - first arg only
			"Error":  true, // some loggers use .Error("message")
			"Warn":   true,
			"Warnf":  true,
			"Info":   true,
			"Infof":  true,
			"Debug":  true,
			"Debugf": true,
			"Trace":  true,
			"Tracef": true,
			"Fatal":  true,
			"Fatalf": true,
			"Panic":  true,
			"Panicf": true,
		}

		if messageMethods[methodName] {
			// Check if our literal is an argument to this method
			for idx, arg := range call.Args {
				if arg == lit {
					// For Str() method, only first argument is a translatable key
					if methodName == "Str" && idx > 0 {
						continue
					}
					return true
				}
			}
		}

		// Check if this is a chained call (e.g., logger.Info().Msg())
		if chainedCall, ok := sel.X.(*ast.CallExpr); ok {
			// Common logger level methods that return a chainable object
			if chainedSel, ok := chainedCall.Fun.(*ast.SelectorExpr); ok {
				levelMethods := map[string]bool{
					"Info":      true,
					"Error":     true,
					"Err":       true, // zerolog style
					"Warn":      true,
					"Debug":     true,
					"Trace":     true,
					"Fatal":     true,
					"Panic":     true,
					"WithLevel": true,
					"Log":       true,
				}

				if levelMethods[chainedSel.Sel.Name] && messageMethods[methodName] {
					// This is a chained logging call
					for idx, arg := range call.Args {
						if arg == lit {
							// For Str() method, only first argument is a translatable key
							if methodName == "Str" && idx > 0 {
								continue
							}
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// ApplyReplacements applies all collected replacements
func (sr *TransformationReplacer) ApplyReplacements() error {
	// Create backup directory
	if err := os.MkdirAll(sr.config.BackupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	if sr.config.IsUpdateMode {
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
	if sr.config.IsUpdateMode {
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
	backupPath := filepath.Join(sr.config.BackupDir, backupName)
	return os.WriteFile(backupPath, content, 0644)
}

// createTranslationCall creates a translation function call string
func (sr *TransformationReplacer) createTranslationCall(key, value string) string {
	// Convert key to Go constant path
	astKey := sr.convertKeyToASTFormat(key)

	pattern := sr.config.TrPattern
	if pattern == "" {
		pattern = "tr.T"
	}

	// For comment mode, indicate format strings need arguments
	if sr.config.TrPattern == "" && strings.Contains(value, "%") {
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

	// Extract package name from path
	// Examples:
	// "./messages" -> "messages"
	// "messages" -> "messages"
	// "github.com/user/project/messages" -> "messages"
	packageName := sr.config.PackagePath
	if strings.HasPrefix(packageName, "./") {
		packageName = packageName[2:]
	}
	// For any path with slashes (module paths or absolute paths), use just the last component
	if strings.Contains(packageName, "/") {
		parts := strings.Split(packageName, "/")
		packageName = parts[len(parts)-1]
	}

	return packageName + ".Keys." + strings.Join(astParts, ".")
}

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

// walkASTWithParents walks the AST while tracking parent nodes
func (sr *TransformationReplacer) walkASTWithParents(node ast.Node, visit func(ast.Node, []ast.Node) bool) {
	var parents []ast.Node

	var walk func(ast.Node) bool
	walk = func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// Call the visitor function
		if !visit(n, parents) {
			return false
		}

		// Add current node to parent stack for children
		parents = append(parents, n)
		defer func() {
			parents = parents[:len(parents)-1]
		}()

		// Walk children based on node type
		switch x := n.(type) {
		case *ast.File:
			for _, decl := range x.Decls {
				walk(decl)
			}
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				walk(spec)
			}
		case *ast.FuncDecl:
			if x.Recv != nil {
				for _, field := range x.Recv.List {
					walk(field.Type)
				}
			}
			walk(x.Type)
			if x.Body != nil {
				walk(x.Body)
			}
		case *ast.BlockStmt:
			for _, stmt := range x.List {
				walk(stmt)
			}
		case *ast.CallExpr:
			walk(x.Fun)
			for _, arg := range x.Args {
				walk(arg)
			}
		case *ast.ReturnStmt:
			for _, result := range x.Results {
				walk(result)
			}
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				walk(lhs)
			}
			for _, rhs := range x.Rhs {
				walk(rhs)
			}
		case *ast.ExprStmt:
			walk(x.X)
		case *ast.IfStmt:
			if x.Init != nil {
				walk(x.Init)
			}
			walk(x.Cond)
			walk(x.Body)
			if x.Else != nil {
				walk(x.Else)
			}
		case *ast.BinaryExpr:
			walk(x.X)
			walk(x.Y)
		case *ast.UnaryExpr:
			walk(x.X)
		case *ast.ParenExpr:
			walk(x.X)
		case *ast.SelectorExpr:
			walk(x.X)
		case *ast.IndexExpr:
			walk(x.X)
			walk(x.Index)
		case *ast.CompositeLit:
			if x.Type != nil {
				walk(x.Type)
			}
			for _, elt := range x.Elts {
				walk(elt)
			}
		case *ast.KeyValueExpr:
			walk(x.Key)
			walk(x.Value)
		case *ast.ForStmt:
			if x.Init != nil {
				walk(x.Init)
			}
			if x.Cond != nil {
				walk(x.Cond)
			}
			if x.Post != nil {
				walk(x.Post)
			}
			walk(x.Body)
		case *ast.RangeStmt:
			if x.Key != nil {
				walk(x.Key)
			}
			if x.Value != nil {
				walk(x.Value)
			}
			walk(x.X)
			walk(x.Body)
		case *ast.SwitchStmt:
			if x.Init != nil {
				walk(x.Init)
			}
			if x.Tag != nil {
				walk(x.Tag)
			}
			walk(x.Body)
		case *ast.CaseClause:
			for _, expr := range x.List {
				walk(expr)
			}
			for _, stmt := range x.Body {
				walk(stmt)
			}
		case *ast.ValueSpec:
			if x.Type != nil {
				walk(x.Type)
			}
			for _, value := range x.Values {
				walk(value)
			}
		case *ast.FieldList:
			if x != nil {
				for _, field := range x.List {
					walk(field.Type)
				}
			}
		case *ast.Field:
			walk(x.Type)
		case *ast.DeferStmt:
			walk(x.Call)
		case *ast.GoStmt:
			walk(x.Call)
		case *ast.FuncLit:
			if x.Type != nil {
				walk(x.Type)
			}
			if x.Body != nil {
				walk(x.Body)
			}
			// Add more node types as needed
		}

		return true
	}

	walk(node)
}

// getFunctionName extracts the function name from a call expression
func (sr *TransformationReplacer) getFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// Handle pkg.Function or receiver.Method
		if ident, ok := fun.X.(*ast.Ident); ok {
			return ident.Name + "." + fun.Sel.Name
		}
	case *ast.Ident:
		// Handle local function calls
		return fun.Name
	}
	return ""
}

// createCommentForContext creates more helpful comment based on context
func (sr *TransformationReplacer) createCommentForContext(key, value string, lit *ast.BasicLit) string {
	// Check if we're in a format function context
	if sr.isInFormatFunction(lit) {
		// For format strings, we need to show the arguments should go inside tr.T()
		// and the format function should change to non-format version
		return sr.createFormatFunctionComment(key, value, lit)
	}

	// For regular strings, just show the simple transformation
	astKey := sr.convertKeyToASTFormat(key)
	pattern := sr.config.TrPattern
	if pattern == "" {
		pattern = "tr.T"
	}
	return fmt.Sprintf("%s(%s)", pattern, astKey)
}

// isInFormatFunction checks if the literal is in a format function call
func (sr *TransformationReplacer) isInFormatFunction(lit *ast.BasicLit) bool {
	// Walk up the parent stack to find if we're in a format function
	for i := len(sr.parentStack) - 1; i >= 0; i-- {
		parent := sr.parentStack[i]

		if callExpr, ok := parent.(*ast.CallExpr); ok {
			funcName := sr.getFunctionName(callExpr)

			// Check if this is a format function
			if strings.HasSuffix(funcName, "f") || strings.Contains(funcName, "Printf") || strings.Contains(funcName, "Sprintf") {
				// Check if our literal is the format string
				for idx, arg := range callExpr.Args {
					if arg == lit {
						// For Fprintf, format string is second argument
						if strings.Contains(funcName, "Fprintf") && idx == 1 {
							return true
						}
						// For most format functions, format string is first argument
						if !strings.Contains(funcName, "Fprintf") && idx == 0 {
							return true
						}
					}
				}
			}

			// Check for chained method calls with format methods
			if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				methodName := sel.Sel.Name
				if strings.HasSuffix(methodName, "f") {
					// Check if our literal is the format string (first argument)
					if len(callExpr.Args) > 0 && callExpr.Args[0] == lit {
						return true
					}
				}
			}
		}
	}

	return false
}

// createFormatFunctionComment creates a comment showing how to transform format functions
func (sr *TransformationReplacer) createFormatFunctionComment(key, value string, lit *ast.BasicLit) string {
	astKey := sr.convertKeyToASTFormat(key)
	pattern := sr.config.TrPattern
	if pattern == "" {
		pattern = "tr.T"
	}

	// Find the parent call expression to get the arguments
	var formatCall *ast.CallExpr
	var formatFuncName string

	for i := len(sr.parentStack) - 1; i >= 0; i-- {
		if call, ok := sr.parentStack[i].(*ast.CallExpr); ok {
			// Check if this call contains our literal as the format string
			for idx, arg := range call.Args {
				if arg == lit {
					formatCall = call
					formatFuncName = sr.getSimpleFunctionName(call)
					// Verify this is the format string position
					if strings.Contains(formatFuncName, "Fprintf") && idx != 1 {
						continue
					}
					if !strings.Contains(formatFuncName, "Fprintf") && idx != 0 {
						continue
					}
					break
				}
			}
			if formatCall != nil {
				break
			}
		}
	}

	if formatCall == nil || formatFuncName == "" {
		// Fallback to simple format
		return fmt.Sprintf("%s(%s, ...)", pattern, astKey)
	}

	// Get the suggested non-format function name
	nonFormatFunc := sr.getNonFormatFunctionName(formatFuncName)

	// Count the number of format arguments (other than the format string)
	numArgs := 0
	for _, arg := range formatCall.Args {
		if arg != lit {
			numArgs++
		}
	}

	if numArgs == 0 {
		// No arguments, just show the simple transformation
		return fmt.Sprintf("%s(%s)", pattern, astKey)
	}

	// Build a comment that shows arguments should go inside tr.T()
	// and the function should change from Msgf to Msg
	argPlaceholders := make([]string, numArgs)
	for i := 0; i < numArgs; i++ {
		argPlaceholders[i] = "arg" + fmt.Sprintf("%d", i+1)
	}

	comment := fmt.Sprintf("%s(%s, %s)", pattern, astKey, strings.Join(argPlaceholders, ", "))

	// If we know the function should change, add that info
	if nonFormatFunc != "" && nonFormatFunc != formatFuncName {
		comment += " and change " + formatFuncName + " to " + nonFormatFunc
	}

	return comment
}

// getSimpleFunctionName gets just the function/method name without package
func (sr *TransformationReplacer) getSimpleFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

// getNonFormatFunctionName returns the non-format version of a format function
func (sr *TransformationReplacer) getNonFormatFunctionName(formatFunc string) string {
	// Common transformations
	switch formatFunc {
	case "Printf":
		return "Print"
	case "Sprintf":
		return "Sprint"
	case "Fprintf":
		return "Fprint"
	case "Errorf":
		return "Error"
	case "Fatalf":
		return "Fatal"
	case "Panicf":
		return "Panic"
	case "Warnf":
		return "Warn"
	case "Infof":
		return "Info"
	case "Debugf":
		return "Debug"
	case "Tracef":
		return "Trace"
	case "Msgf":
		return "Msg"
	case "Logf":
		return "Log"
	}

	// Generic handling: remove trailing 'f'
	if strings.HasSuffix(formatFunc, "f") && len(formatFunc) > 1 {
		return formatFunc[:len(formatFunc)-1]
	}

	return formatFunc
}
