package ast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FormatTransformer handles AST transformation of format function calls
type FormatTransformer struct {
	fset               *token.FileSet
	stringMap          map[string]string    // maps string literals to translation keys
	requiredImports    map[string]bool      // tracks imports needed
	transformed        bool                 // tracks if any transformations were made
	detector           *FormatDetector      // generic format function detector
	packagePath        string               // path to the messages package
	transformMode      string               // "user-facing", "with-comments", "all-marked", "all"
	i18nTodoMap        map[token.Pos]string // maps string literal positions to i18n-todo message keys
	userFacingRegexes  []*regexp.Regexp     // regex patterns to identify user-facing functions
	skipPositions      map[token.Pos]bool   // positions of strings that should be skipped due to i18n-skip comments
	trPattern          string               // translator pattern (e.g., "tr.T")
	currentFilename    string               // current filename being processed
	trDeclaredInPkg    bool                 // whether TR is already declared in another file in the package
	transformedStrings map[string]string    // tracks which strings were actually transformed (string value -> key)
}

// NewFormatTransformer creates a new format transformer
func NewFormatTransformer(stringMap map[string]string) *FormatTransformer {
	return &FormatTransformer{
		fset:               token.NewFileSet(),
		stringMap:          stringMap,
		requiredImports:    make(map[string]bool),
		transformed:        false,
		detector:           NewFormatDetector(),
		packagePath:        "messages",    // default value (will be resolved to full module path)
		transformMode:      "user-facing", // default to only transforming known user-facing functions
		i18nTodoMap:        make(map[token.Pos]string),
		skipPositions:      make(map[token.Pos]bool),
		trPattern:          "tr.T", // default translator pattern
		transformedStrings: make(map[string]string),
	}
}

// SetMessagePackagePath sets the path to the messages package
func (ft *FormatTransformer) SetMessagePackagePath(path string) {
	ft.packagePath = path
}

// SetTransformMode sets the transformation mode
func (ft *FormatTransformer) SetTransformMode(mode string) {
	ft.transformMode = mode
}

// SetTranslatorPattern sets the translator pattern (e.g., "tr.T")
func (ft *FormatTransformer) SetTranslatorPattern(pattern string) {
	ft.trPattern = pattern
}

// SetUserFacingRegexes sets the regex patterns to identify user-facing functions
func (ft *FormatTransformer) SetUserFacingRegexes(patterns []string) error {
	ft.userFacingRegexes = nil
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid user-facing regex '%s': %w", pattern, err)
		}
		ft.userFacingRegexes = append(ft.userFacingRegexes, regex)
	}
	return nil
}

// SetFormatFunctionPatterns registers custom format function patterns
func (ft *FormatTransformer) SetFormatFunctionPatterns(patterns []string) error {
	for _, pattern := range patterns {
		// Parse pattern:index format
		parts := strings.SplitN(pattern, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format function pattern '%s', expected 'pattern:index'", pattern)
		}

		regex := parts[0]
		indexStr := parts[1]

		// Parse the index
		var index int
		if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
			return fmt.Errorf("invalid format arg index '%s' in pattern '%s'", indexStr, pattern)
		}

		// Register with the detector
		if err := ft.detector.RegisterCustomFormatPattern(regex, index); err != nil {
			return fmt.Errorf("failed to register format pattern '%s': %w", pattern, err)
		}
	}
	return nil
}

// GetTransformedStrings returns the map of strings that were actually transformed (string value -> key)
func (ft *FormatTransformer) GetTransformedStrings() map[string]string {
	return ft.transformedStrings
}

// buildSkipPositions scans the AST to find string literals with i18n-skip comments
// and ensures those comments are properly attached to the AST
func (ft *FormatTransformer) buildSkipPositions(file *ast.File) {
	// Clear the map
	ft.skipPositions = make(map[token.Pos]bool)

	// Create a map to track which comments need to be preserved
	skipComments := make(map[*ast.Comment]int)         // comment -> line number
	skipCommentsByLine := make(map[int][]*ast.Comment) // line -> comments

	// First, collect all i18n-skip comments and their positions
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			// Check if this is an i18n-skip comment (both line and block comments)
			if strings.Contains(strings.ToLower(c.Text), "i18n-skip") {
				pos := ft.fset.Position(c.Pos())
				skipComments[c] = pos.Line
				skipCommentsByLine[pos.Line] = append(skipCommentsByLine[pos.Line], c)
			}
		}
	}

	// Track nodes that need comments attached
	nodesToAttachComments := make(map[ast.Node][]*ast.Comment)

	// Now walk the AST to find string literals that have i18n-skip comments
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			litPos := ft.fset.Position(lit.Pos())
			litEndPos := ft.fset.Position(lit.End())

			// Check multiple scenarios for i18n-skip comments:

			// 1. Same line comment (after the string literal)
			if comments, exists := skipCommentsByLine[litPos.Line]; exists {
				for _, comment := range comments {
					commentPos := ft.fset.Position(comment.Pos())
					// Ensure comment is after the string literal on the same line
					// AND before the next argument (if any)
					if commentPos.Column > litEndPos.Column {
						// Check if this is really for this string literal
						// by verifying there's no other string literal between this one and the comment
						isForThisLiteral := true

						// Walk all nodes to find if there's another string between this literal and the comment
						ast.Inspect(file, func(n ast.Node) bool {
							if otherLit, ok := n.(*ast.BasicLit); ok && otherLit.Kind == token.STRING && otherLit != lit {
								otherPos := ft.fset.Position(otherLit.Pos())
								otherEndPos := ft.fset.Position(otherLit.End())
								// Check if the other literal is on the same line and between our literal and the comment
								if otherPos.Line == litPos.Line &&
									otherEndPos.Column > litEndPos.Column &&
									otherPos.Column < commentPos.Column {
									isForThisLiteral = false
									return false
								}
							}
							return true
						})

						if isForThisLiteral {
							ft.skipPositions[lit.Pos()] = true

							// Find the parent node to attach the comment to
							parentNode := ft.findParentNode(file, lit)
							if parentNode != nil {
								nodesToAttachComments[parentNode] = append(nodesToAttachComments[parentNode], comment)
							}
							break
						}
					}
				}
			}

			// 2. Previous line comment (comment on the line before)
			// This handles cases like:
			// // i18n-skip
			// msg := "string"
			if litPos.Line > 1 {
				// Check comments on the immediate previous line
				if comments, exists := skipCommentsByLine[litPos.Line-1]; exists {
					// Only apply if the comment is a standalone line comment (not inline)
					for _, comment := range comments {
						commentPos := ft.fset.Position(comment.Pos())
						// Check if this is a standalone comment (typically starts near beginning of line)
						// Inline comments would have been handled in case 1 above
						isStandaloneComment := false

						// If it's a line comment starting with //, check if it's at the beginning
						if strings.HasPrefix(comment.Text, "//") && commentPos.Column < 20 {
							isStandaloneComment = true
						}
						// Block comments at line start are also standalone
						if strings.HasPrefix(comment.Text, "/*") && commentPos.Column < 10 {
							isStandaloneComment = true
						}

						if isStandaloneComment {
							ft.skipPositions[lit.Pos()] = true

							// Find the parent node to attach the comment to
							parentNode := ft.findParentNode(file, lit)
							if parentNode != nil {
								nodesToAttachComments[parentNode] = append(nodesToAttachComments[parentNode], comment)
							}
							break
						}
					}
				}
			}
			// 3. Check for skip comments in multi-line contexts
			// For example, in a function call that spans multiple lines:
			// // i18n-skip
			// fmt.Println(
			//     "string to skip"
			// )
			if parentCall := ft.findParentCall(file, lit); parentCall != nil {
				callPos := ft.fset.Position(parentCall.Pos())
				// Check if there's a skip comment on the line before the call
				if callPos.Line > 1 {
					if comments, exists := skipCommentsByLine[callPos.Line-1]; exists {
						for _, comment := range comments {
							commentPos := ft.fset.Position(comment.Pos())
							// Only apply if it's a standalone comment, not an inline comment
							isStandaloneComment := false

							// If it's a line comment starting with //, check if it's at the beginning
							if strings.HasPrefix(comment.Text, "//") && commentPos.Column < 20 {
								isStandaloneComment = true
							}
							// Block comments at line start are also standalone
							if strings.HasPrefix(comment.Text, "/*") && commentPos.Column < 10 {
								isStandaloneComment = true
							}

							if isStandaloneComment {
								ft.skipPositions[lit.Pos()] = true
								nodesToAttachComments[parentCall] = append(nodesToAttachComments[parentCall], comment)
								break
							}
						}
					}
				}
			}
		}
		return true
	})

	// Ensure skip comments are properly attached to their nodes
	ft.attachCommentsToNodes(file, nodesToAttachComments)
}

// RemoveI18nTodoCommentsFromSource removes i18n-todo comments from source code
// This is done before AST parsing to avoid issues with embedded comments
func (ft *FormatTransformer) RemoveI18nTodoCommentsFromSource(src []byte) []byte {
	// We need to track which strings had i18n-todo comments for later transformation
	lines := strings.Split(string(src), "\n")
	var result []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Remove block comments /* i18n-todo: ... */
		for {
			startIdx := strings.Index(line, "/"+"* i18n-todo:")
			if startIdx < 0 {
				break
			}
			endIdx := strings.Index(line[startIdx:], "*"+"/")
			if endIdx < 0 {
				// Malformed comment, skip
				break
			}
			// Remove the comment
			line = line[:startIdx] + line[startIdx+endIdx+2:]
		}

		// Remove line comments // i18n-todo: ...
		if idx := strings.Index(line, "// i18n-todo:"); idx >= 0 {
			line = strings.TrimRight(line[:idx], " \t")
		}

		result = append(result, line)
	}

	return []byte(strings.Join(result, "\n"))
}

// TransformFile transforms format functions in a file
func (ft *FormatTransformer) TransformFile(filename string, src []byte) ([]byte, error) {
	// Store the current filename
	ft.currentFilename = filename
	// Reset the package-level TR declaration flag for each file
	ft.trDeclaredInPkg = false

	// First pass: Remove i18n-todo comments from the source before parsing
	// This prevents AST parsing issues with embedded comments
	if ft.transformMode == "with-comments" || ft.transformMode == "all-marked" {
		src = ft.RemoveI18nTodoCommentsFromSource(src)
	}

	// Parse the file
	file, err := parser.ParseFile(ft.fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Build skip positions map - needed for all transform modes
	ft.buildSkipPositions(file)

	// Transform the AST based on mode
	switch ft.transformMode {
	case "user-facing":
		// Only transform user-facing functions
		ast.Inspect(file, ft.transformNode)
	case "with-comments":
		// In this mode, only transform strings that have translation keys
		// The caller should have filtered the stringMap to only include strings with i18n-todo comments
		ft.transformAllStrings(file)
	case "all-marked":
		// Transform both user-facing AND all strings (since i18n-todo comments are removed)
		ast.Inspect(file, ft.transformNode)
		ft.transformAllStrings(file)
	case "all":
		// Transform all strings that have keys
		ft.transformAllStrings(file)
	default:
		// Default to user-facing only
		ast.Inspect(file, ft.transformNode)
	}

	// Add required imports if any transformations were made
	if ft.transformed {
		ft.addImports(file)
	}

	// Convert back to source
	result, err := ft.formatNode(file)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// transformAllStrings transforms all string literals that have translation keys
func (ft *FormatTransformer) transformAllStrings(file *ast.File) {
	// First pass: identify which ValueSpecs are in const declarations
	constValueSpecs := make(map[*ast.ValueSpec]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					constValueSpecs[valueSpec] = true
				}
			}
		}
		return true
	})

	// Second pass: transform strings
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			// Handle format functions and regular function calls
			ft.transformNode(x)
		case *ast.AssignStmt:
			// Handle assignments
			for i, rhs := range x.Rhs {
				if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					// Check if this string has an i18n-skip comment
					if ft.skipPositions[lit.Pos()] {
						continue // Skip transformation
					}
					for quotedStr, key := range ft.stringMap {
						if lit.Value == quotedStr {
							trCall := ft.createTrCall(key, nil)
							if trCall != nil {
								x.Rhs[i] = trCall
								ft.transformed = true
								ft.requiredImports["messages"] = true
								ft.requiredImports["i18n"] = true
								// Track the transformed string (unquoted value)
								unquotedStr := strings.Trim(quotedStr, "`\"")
								ft.transformedStrings[unquotedStr] = key
							}
							break
						}
					}
				}
			}
		case *ast.ValueSpec:
			// Handle var/const declarations
			// IMPORTANT: Skip transformation for const declarations as they can't call functions
			if constValueSpecs[x] {
				return true // Skip const transformations entirely
			}

			for i, val := range x.Values {
				if lit, ok := val.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					// Check if this string has an i18n-skip comment
					if ft.skipPositions[lit.Pos()] {
						continue // Skip transformation
					}
					for quotedStr, key := range ft.stringMap {
						if lit.Value == quotedStr {
							trCall := ft.createTrCall(key, nil)
							if trCall != nil {
								x.Values[i] = trCall
								ft.transformed = true
								ft.requiredImports["messages"] = true
								ft.requiredImports["i18n"] = true
							}
							break
						}
					}
				}
			}
		case *ast.ReturnStmt:
			// Handle return statements
			for i, result := range x.Results {
				if lit, ok := result.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					// Check if this string has an i18n-skip comment
					if ft.skipPositions[lit.Pos()] {
						continue // Skip transformation
					}
					for quotedStr, key := range ft.stringMap {
						if lit.Value == quotedStr {
							trCall := ft.createTrCall(key, nil)
							if trCall != nil {
								x.Results[i] = trCall
								ft.transformed = true
								ft.requiredImports["messages"] = true
								ft.requiredImports["i18n"] = true
								// Track the transformed string (unquoted value)
								unquotedStr := strings.Trim(quotedStr, "`\"")
								ft.transformedStrings[unquotedStr] = key
							}
							break
						}
					}
				}
			}
		case *ast.CompositeLit:
			// Handle composite literals (struct/slice/map initialization)
			for i, elt := range x.Elts {
				switch e := elt.(type) {
				case *ast.KeyValueExpr:
					// Handle key-value pairs in structs/maps
					if lit, ok := e.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						// Check if this string has an i18n-skip comment
						if ft.skipPositions[lit.Pos()] {
							continue // Skip transformation
						}
						for quotedStr, key := range ft.stringMap {
							if lit.Value == quotedStr {
								trCall := ft.createTrCall(key, nil)
								if trCall != nil {
									e.Value = trCall
									ft.transformed = true
									ft.requiredImports["messages"] = true
									ft.requiredImports["i18n"] = true
									// Track the transformed string (unquoted value)
									unquotedStr := strings.Trim(quotedStr, "`\"")
									ft.transformedStrings[unquotedStr] = key
								}
								break
							}
						}
					}
				case *ast.BasicLit:
					// Handle array/slice elements
					if e.Kind == token.STRING {
						// Check if this string has an i18n-skip comment
						if ft.skipPositions[e.Pos()] {
							continue // Skip transformation
						}
						for quotedStr, key := range ft.stringMap {
							if e.Value == quotedStr {
								trCall := ft.createTrCall(key, nil)
								if trCall != nil {
									x.Elts[i] = trCall
									ft.transformed = true
									ft.requiredImports["messages"] = true
									ft.requiredImports["i18n"] = true
									// Track the transformed string (unquoted value)
									unquotedStr := strings.Trim(quotedStr, "`\"")
									ft.transformedStrings[unquotedStr] = key
								}
								break
							}
						}
					}
				}
			}
		}
		return true
	})
}

// transformNode examines and transforms individual AST nodes
func (ft *FormatTransformer) transformNode(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return true
	}

	// First try format functions
	formatInfo := ft.detector.DetectFormatCall(call)
	if formatInfo != nil {
		// Check if the format string has an i18n-skip comment
		if ft.skipPositions[call.Args[formatInfo.FormatStringIndex].Pos()] {
			return true // Skip transformation
		}
		// Check if we have a translation key for this string
		quotedStr := fmt.Sprintf("%q", formatInfo.FormatString)
		key, exists := ft.stringMap[quotedStr]
		if !exists {
			return true // No translation key for this string
		}

		// Get transformation type
		transformType := ft.detector.SuggestTransformation(formatInfo)

		// Transform based on type
		switch transformType {
		case "Print":
			ft.transformGenericPrintf(call, formatInfo, key)
		case "Direct":
			ft.transformGenericDirect(call, formatInfo, key)
		case "Fprint":
			ft.transformGenericFprintf(call, formatInfo, key)
		case "Error":
			ft.transformGenericErrorf(call, formatInfo, key)
		case "Wrapf":
			ft.transformGenericWrapf(call, formatInfo, key)
		default:
			// For unknown patterns, try generic transformation
			if formatInfo.IsVariadic {
				ft.transformGenericPrintf(call, formatInfo, key)
			}
		}

		ft.transformed = true
		ft.requiredImports["messages"] = true
		// Track the transformed string (unquoted value)
		unquotedStr := strings.Trim(quotedStr, "`\"")
		ft.transformedStrings[unquotedStr] = key
		return true
	}

	// If not a format function, check for regular function calls with string literals
	ft.transformRegularFunctionCall(call)

	return true
}

// createTrCall creates a tr.T function call expression
func (ft *FormatTransformer) createTrCall(key string, args []ast.Expr) *ast.CallExpr {
	// Parse the translator pattern to build the correct AST
	// Examples: "tr.T", "CGG.TR().T", "i18n.Translate", etc.
	fun := ft.parseTranslatorPattern()

	return &ast.CallExpr{
		Fun:  fun,
		Args: ft.createTrCallArgs(key, args),
	}
}

// parseTranslatorPattern parses the translator pattern string into an AST expression
func (ft *FormatTransformer) parseTranslatorPattern() ast.Expr {
	// Parse the pattern as a Go expression
	expr, err := parser.ParseExpr(ft.trPattern)
	if err != nil {
		// Fallback to default if pattern is invalid
		return &ast.SelectorExpr{
			X:   ast.NewIdent("tr"),
			Sel: ast.NewIdent("T"),
		}
	}

	// Return the parsed expression as-is to preserve the pattern exactly
	// Examples:
	// - "tr.T" → SelectorExpr
	// - "TR().t" → SelectorExpr with CallExpr as X
	// - "CGG.TR().T" → SelectorExpr with nested calls
	return expr
}

// removeTrailingCalls removes any trailing empty call expressions from the AST
// For example, "CGG.TR().T" would have TR() as a call, but we want just the selector
func removeTrailingCalls(expr ast.Expr) ast.Expr {
	switch e := expr.(type) {
	case *ast.CallExpr:
		// If it's a call with no arguments, check if the Fun is what we want
		if len(e.Args) == 0 {
			return removeTrailingCalls(e.Fun)
		}
		// Otherwise, keep the call but process its Fun
		e.Fun = removeTrailingCalls(e.Fun)
		return e
	case *ast.SelectorExpr:
		// Process the X part in case it has calls
		e.X = removeTrailingCalls(e.X)
		return e
	default:
		// For other expression types, return as-is
		return expr
	}
}

// createTrCallArgs creates the arguments for a tr.T call
func (ft *FormatTransformer) createTrCallArgs(key string, formatArgs []ast.Expr) []ast.Expr {
	args := make([]ast.Expr, 0, len(formatArgs)+1)

	// Add the key
	args = append(args, ft.createKeyExpr(key))

	// Add format arguments
	args = append(args, formatArgs...)

	return args
}

// createKeyExpr creates the AST expression for a message key
func (ft *FormatTransformer) createKeyExpr(key string) ast.Expr {
	// The input key is in clean format (e.g., "app.extracted.failed_to_disable")
	// We need to convert it to AST format (e.g., "messages.Keys.App.Extracted.FailedToDisable")

	// First, convert the key to Go naming convention
	goKey := ft.keyToGoName(key)

	// Then prepend the package path and "Keys."
	packageName := ft.packagePath
	if packageName == "" {
		packageName = "messages"
	}

	// Extract just the package name from the path
	// For paths like "github.com/user/project/messages", use just "messages"
	if strings.Contains(packageName, "/") {
		pathParts := strings.Split(packageName, "/")
		packageName = pathParts[len(pathParts)-1]
	}

	astKey := packageName + ".Keys." + goKey
	parts := strings.Split(astKey, ".")

	// Start with the first part
	expr := ast.Expr(ast.NewIdent(parts[0]))

	// Build selector expression for each remaining part
	for i := 1; i < len(parts); i++ {
		expr = &ast.SelectorExpr{
			X:   expr,
			Sel: ast.NewIdent(parts[i]),
		}
	}

	return expr
}

// keyToGoName converts a translation key to a valid Go identifier
// This mirrors the logic in util.KeyToGoName
func (ft *FormatTransformer) keyToGoName(key string) string {
	// Handle the full key path (e.g., "app.extracted.failed_to_disable")
	parts := strings.Split(key, ".")

	var result []string
	for _, part := range parts {
		result = append(result, ft.partToGoName(part))
	}

	return strings.Join(result, ".")
}

// partToGoName converts a single part of a key to a valid Go identifier
func (ft *FormatTransformer) partToGoName(s string) string {
	if s == "" {
		return ""
	}

	// Replace common separators with underscores
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")

	// Split by underscores and capitalize
	parts := strings.Split(s, "_")
	var result []string

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Ensure it doesn't start with a number
		if len(part) > 0 && part[0] >= '0' && part[0] <= '9' {
			part = "N" + part // Prefix with 'N' for "Number"
		}

		// Capitalize first letter
		if len(part) > 0 {
			part = strings.ToUpper(part[:1]) + part[1:]
		}

		result = append(result, part)
	}

	return strings.Join(result, "")
}

// addImports adds required imports to the file
func (ft *FormatTransformer) addImports(file *ast.File) {
	// Check which imports we need to add
	needMessages := ft.requiredImports["messages"]
	needErrors := ft.requiredImports["errors"]

	if !needMessages && !needErrors {
		return
	}

	// First, check if we'll need TR initialization
	// This is needed to determine if we should add i18n import
	needI18n := false

	if needMessages {
		// Parse the pattern to see if we need TR
		if expr, err := parser.ParseExpr(ft.trPattern); err == nil {
			var rootIdent string
			switch e := expr.(type) {
			case *ast.SelectorExpr:
				switch x := e.X.(type) {
				case *ast.Ident:
					rootIdent = x.Name
				case *ast.CallExpr:
					if ident, ok := x.Fun.(*ast.Ident); ok {
						rootIdent = ident.Name
					}
				}
			}

			if rootIdent != "" {
				// Check if TR is already declared in this file or another file
				hasInFile := false
				for _, decl := range file.Decls {
					switch d := decl.(type) {
					case *ast.GenDecl:
						if d.Tok == token.VAR || d.Tok == token.CONST {
							for _, spec := range d.Specs {
								if valueSpec, ok := spec.(*ast.ValueSpec); ok {
									for _, name := range valueSpec.Names {
										if name.Name == rootIdent {
											hasInFile = true
											break
										}
									}
								}
							}
						}
					case *ast.FuncDecl:
						if d.Name.Name == rootIdent {
							hasInFile = true
						}
					}
				}

				if !hasInFile && !ft.checkTrDeclaredInPackage(rootIdent) {
					needI18n = true
				} else if hasInFile || ft.checkTrDeclaredInPackage(rootIdent) {
					// TR exists somewhere, don't add i18n import
					needI18n = false
				}
			}
		}
	}

	// Use astutil to add imports - it handles all the edge cases correctly
	if needMessages {
		// Check if messages package is already imported
		if !ft.hasImportPath(file, ft.packagePath) {
			astutil.AddImport(ft.fset, file, ft.packagePath)
		}
	}

	if needErrors {
		// Check if errors package is already imported
		if !ft.hasImportPath(file, "errors") {
			astutil.AddImport(ft.fset, file, "errors")
		}
	}

	if needI18n {
		// Only add i18n import if we'll be adding TR declaration
		// Check if i18n package is already imported
		if !ft.hasImportPath(file, "github.com/napalu/goopt/v2/i18n") {
			astutil.AddImport(ft.fset, file, "github.com/napalu/goopt/v2/i18n")
		}
	}

	// Add tr variable initialization if needed
	if needMessages {
		ft.addTrInitialization(file)
	}
}

// hasImportPath checks if an import path already exists in the file
func (ft *FormatTransformer) hasImportPath(file *ast.File, path string) bool {
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			for _, spec := range genDecl.Specs {
				if importSpec, ok := spec.(*ast.ImportSpec); ok {
					importPath := strings.Trim(importSpec.Path.Value, `"`)
					if importPath == path {
						return true
					}
				}
			}
		}
	}
	return false
}

// checkTrDeclaredInPackage checks if the TR pattern identifier is already declared in other files in the same package
func (ft *FormatTransformer) checkTrDeclaredInPackage(rootIdent string) bool {
	if ft.currentFilename == "" {
		return false
	}

	// Get the directory of the current file
	dir := filepath.Dir(ft.currentFilename)

	// Read all .go files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		// Skip test files
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filename := filepath.Join(dir, entry.Name())
		// Skip the current file
		if filename == ft.currentFilename {
			continue
		}

		// Read and parse the file
		content, err := os.ReadFile(filename)
		if err != nil {
			continue
		}

		// Quick check before parsing - look for the identifier
		if !strings.Contains(string(content), rootIdent) {
			continue
		}

		// Parse the file
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filename, content, 0)
		if err != nil {
			continue
		}

		// Check if the identifier is declared
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok == token.VAR || d.Tok == token.CONST {
					for _, spec := range d.Specs {
						if valueSpec, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range valueSpec.Names {
								if name.Name == rootIdent {
									return true
								}
							}
						}
					}
				}
			case *ast.FuncDecl:
				if d.Name.Name == rootIdent {
					return true
				}
			}
		}
	}

	return false
}

// addTrInitialization adds tr variable declaration/initialization
func (ft *FormatTransformer) addTrInitialization(file *ast.File) {
	// Parse the pattern to determine what needs to be declared
	expr, err := parser.ParseExpr(ft.trPattern)
	if err != nil {
		return // Invalid pattern, skip initialization
	}

	// Extract the root identifier from the pattern
	var rootIdent string
	var needsFunc bool

	switch e := expr.(type) {
	case *ast.SelectorExpr:
		// Pattern like "tr.T" or "TR().T"
		switch x := e.X.(type) {
		case *ast.Ident:
			// Simple case: tr.T
			rootIdent = x.Name
			needsFunc = false
		case *ast.CallExpr:
			// Function call case: TR().T
			if ident, ok := x.Fun.(*ast.Ident); ok {
				rootIdent = ident.Name
				needsFunc = true
			}
		}
	case *ast.CallExpr:
		// Pattern like "TR()"
		if ident, ok := e.Fun.(*ast.Ident); ok {
			rootIdent = ident.Name
			needsFunc = true
		}
	}

	if rootIdent == "" {
		return // Couldn't determine what to declare
	}

	// Check if already declared (both vars and funcs)
	hasDecl := false
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, spec := range d.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.Name == rootIdent {
								hasDecl = true
								break
							}
						}
					}
				}
			}
		case *ast.FuncDecl:
			if d.Name.Name == rootIdent {
				hasDecl = true
			}
		}
	}

	if hasDecl {
		return
	}

	// Check if it's declared in another file in the same package
	if ft.checkTrDeclaredInPackage(rootIdent) {
		ft.trDeclaredInPkg = true
		return
	}

	// Create appropriate declaration based on pattern type
	var decl ast.Decl
	if needsFunc {
		// Create function declaration: var TR = func() i18n.Translator { ... }
		decl = &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(rootIdent)},
					Values: []ast.Expr{
						&ast.FuncLit{
							Type: &ast.FuncType{
								Params: &ast.FieldList{},
								Results: &ast.FieldList{
									List: []*ast.Field{
										{
											Type: &ast.SelectorExpr{
												X:   ast.NewIdent("i18n"),
												Sel: ast.NewIdent("Translator"),
											},
										},
									},
								},
							},
							Body: &ast.BlockStmt{
								List: []ast.Stmt{
									// Use panic to make it very clear this needs implementation
									&ast.ExprStmt{
										X: &ast.CallExpr{
											Fun: ast.NewIdent("panic"),
											Args: []ast.Expr{
												&ast.BasicLit{
													Kind:  token.STRING,
													Value: `"TODO: Implement TR() - return your i18n.Translator instance"`,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
	} else {
		// Create variable declaration: var tr i18n.Translator
		decl = &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(rootIdent)},
					Type: &ast.SelectorExpr{
						X:   ast.NewIdent("i18n"),
						Sel: ast.NewIdent("Translator"),
					},
				},
			},
		}
	}

	// Find a safe position to insert the declaration
	// Simple strategy: insert right after imports
	insertPos := -1

	// Find the last import declaration
	for i := len(file.Decls) - 1; i >= 0; i-- {
		if genDecl, ok := file.Decls[i].(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			insertPos = i + 1
			break
		}
	}

	// If no imports found, insert after package
	if insertPos == -1 {
		for i, d := range file.Decls {
			if genDecl, ok := d.(*ast.GenDecl); ok && genDecl.Tok == token.PACKAGE {
				insertPos = i + 1
				break
			}
		}
	}

	// If still not found, just insert at position 1
	if insertPos == -1 {
		insertPos = 1
	}

	// Insert the declaration
	newDecls := make([]ast.Decl, 0, len(file.Decls)+1)
	newDecls = append(newDecls, file.Decls[:insertPos]...)
	newDecls = append(newDecls, decl)
	newDecls = append(newDecls, file.Decls[insertPos:]...)
	file.Decls = newDecls
}

// Generic transformation methods that work with FormatCallInfo

// transformGenericPrintf handles Printf-style functions generically
func (ft *FormatTransformer) transformGenericPrintf(call *ast.CallExpr, info *FormatCallInfo, key string) {
	// Smart detection: determine transformation strategy based on format string position
	// and the presence of additional arguments after it

	// Strategy 1: Classical Printf pattern (format at position 0)
	// - fmt.Printf("format %s", args...) -> fmt.Print(tr.T(key, args...))
	// - Remove 'f' suffix and replace ALL arguments

	// Strategy 2: Writer-based pattern (format at position 1)
	// - fmt.Fprintf(w, "format %s", args...) -> fmt.Fprint(w, tr.T(key, args...))
	// - Keep writer, remove 'f', replace format and args

	// Strategy 3: Custom format function (format at any position with args after)
	// - s.Log.MsgAll(map, "format %s", args...) -> s.Log.MsgAll(map, tr.T(key, args...))
	// - Keep function name, replace format string and consume variadic args

	// Determine the transformation strategy
	isClassicalPrintf := false
	isWriterBased := false
	hasArgsAfterFormat := info.FormatStringIndex < len(call.Args)-1

	// Check if it's a standard Printf-style function
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		method := sel.Sel.Name

		// Check for any method ending with 'f' that we should transform
		if strings.HasSuffix(method, "f") && info.FormatStringIndex == 0 {
			// Always treat format functions ending with 'f' as classical printf
			// This includes Msgf, Infof, Printf, etc.
			isClassicalPrintf = true
		} else if info.FormatStringIndex == 1 {
			// Check for writer-based patterns
			if ident, ok := sel.X.(*ast.Ident); ok {
				pkg := ident.Name
				if pkg == "fmt" && strings.HasPrefix(method, "Fprint") {
					isWriterBased = true
				}
			}
		}
	}

	// Apply transformation based on detected pattern
	if isClassicalPrintf {
		// Classical Printf: remove 'f' and replace ALL arguments
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if strings.HasSuffix(sel.Sel.Name, "f") {
				sel.Sel = ast.NewIdent(strings.TrimSuffix(sel.Sel.Name, "f"))
			}
		}

		args := ft.extractFormatArgs(call, info)
		trCall := ft.createTrCall(key, args)
		call.Args = []ast.Expr{trCall}

		// Clear the Lparen/Rparen positions to let the formatter recreate them
		call.Lparen = 0
		call.Rparen = 0

	} else if isWriterBased {
		// Writer-based: keep writer, remove 'f', replace format+args
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if strings.HasSuffix(sel.Sel.Name, "f") {
				sel.Sel = ast.NewIdent(strings.TrimSuffix(sel.Sel.Name, "f"))
			}
		}

		writer := call.Args[0]
		args := ft.extractFormatArgs(call, info)
		trCall := ft.createTrCall(key, args)
		call.Args = []ast.Expr{writer, trCall}

		// Clear the Lparen/Rparen positions to let the formatter recreate them
		call.Lparen = 0
		call.Rparen = 0

	} else if hasArgsAfterFormat {
		// Custom format function with variadic args after format string
		// Replace format string and all subsequent args with tr.T call
		args := ft.extractFormatArgs(call, info)
		trCall := ft.createTrCall(key, args)

		// Keep all args before format string, replace format and all args after
		newArgs := make([]ast.Expr, 0, info.FormatStringIndex+1)
		for i := 0; i < info.FormatStringIndex; i++ {
			newArgs = append(newArgs, call.Args[i])
		}
		newArgs = append(newArgs, trCall)
		call.Args = newArgs

	} else {
		// Custom format function with no args after format string
		// Just replace the format string argument
		trCall := ft.createTrCall(key, nil)
		call.Args[info.FormatStringIndex] = trCall
	}
}

// transformGenericDirect handles direct replacement (like Sprintf)
func (ft *FormatTransformer) transformGenericDirect(call *ast.CallExpr, info *FormatCallInfo, key string) {
	// Replace the entire call with tr.T
	call.Fun = &ast.SelectorExpr{
		X:   ast.NewIdent("tr"),
		Sel: ast.NewIdent("T"),
	}

	// Create new arguments: key + format args
	args := ft.extractFormatArgs(call, info)
	call.Args = ft.createTrCallArgs(key, args)
}

// transformGenericFprintf handles Fprintf-style functions
func (ft *FormatTransformer) transformGenericFprintf(call *ast.CallExpr, info *FormatCallInfo, key string) {
	// Change Fprintf to Fprint
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		funcName := sel.Sel.Name
		if strings.HasSuffix(funcName, "f") {
			sel.Sel = ast.NewIdent(strings.TrimSuffix(funcName, "f"))
		}
	}

	// Extract writer and format args
	var writer ast.Expr
	var formatArgs []ast.Expr

	for i, arg := range call.Args {
		if i < info.FormatStringIndex {
			writer = arg // Assume first non-format arg is writer
		} else if i > info.FormatStringIndex {
			formatArgs = append(formatArgs, arg)
		}
	}

	// Create tr.T call
	trCall := ft.createTrCall(key, formatArgs)

	// New args: writer + tr.T result
	if writer != nil {
		call.Args = []ast.Expr{writer, trCall}
	} else {
		call.Args = []ast.Expr{trCall}
	}
}

// transformGenericErrorf handles Errorf-style functions
func (ft *FormatTransformer) transformGenericErrorf(call *ast.CallExpr, info *FormatCallInfo, key string) {
	// Check for error wrapping
	hasErrorWrap := strings.Contains(info.FormatString, "%w")

	if hasErrorWrap {
		// Preserve error wrapping
		// For mixed cases like "failed to connect to %s: %w", we need to handle
		// the format args properly

		// Count the number of format specifiers excluding %w
		nonWrapSpecifiers := 0
		for _, match := range regexp.MustCompile(`%[^w%]`).FindAllString(info.FormatString, -1) {
			if match != "%%" { // Skip escaped %
				nonWrapSpecifiers++
			}
		}

		// Extract format args
		var formatArgs []ast.Expr
		var errorArg ast.Expr

		argCount := 0
		for i, arg := range call.Args {
			if i > info.FormatStringIndex {
				if argCount < nonWrapSpecifiers {
					// These are the regular format args
					formatArgs = append(formatArgs, arg)
					argCount++
				} else {
					// This should be the error arg for %w
					errorArg = arg
				}
			}
		}

		// Create tr.T call
		trCall := ft.createTrCall(key, formatArgs)

		// Replace format string with "%s: %w"
		call.Args[info.FormatStringIndex] = &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"%s: %w"`,
		}

		// New args
		newArgs := make([]ast.Expr, 0, info.FormatStringIndex+3)
		for i := 0; i < info.FormatStringIndex; i++ {
			newArgs = append(newArgs, call.Args[i])
		}
		newArgs = append(newArgs, call.Args[info.FormatStringIndex], trCall)
		if errorArg != nil {
			newArgs = append(newArgs, errorArg)
		}
		call.Args = newArgs
	} else {
		// No error wrapping - convert to errors.New
		call.Fun = &ast.SelectorExpr{
			X:   ast.NewIdent("errors"),
			Sel: ast.NewIdent("New"),
		}

		args := ft.extractFormatArgs(call, info)
		trCall := ft.createTrCall(key, args)
		call.Args = []ast.Expr{trCall}

		// Clear the Lparen/Rparen positions to let the formatter recreate them
		call.Lparen = 0
		call.Rparen = 0

		ft.requiredImports["errors"] = true
	}
}

// transformGenericWrapf handles Wrapf-style functions (errors.Wrapf)
func (ft *FormatTransformer) transformGenericWrapf(call *ast.CallExpr, info *FormatCallInfo, key string) {
	// For errors.Wrapf(err, "format %s", arg), transform to errors.Wrapf(err, "%s", tr.T(key, arg))
	// Keep the function name as-is, just replace format string with "%s" and create tr.T call

	// Replace format string with "%s"
	call.Args[info.FormatStringIndex] = &ast.BasicLit{
		Kind:  token.STRING,
		Value: `"%s"`,
	}

	// Extract format arguments (everything after the format string)
	var formatArgs []ast.Expr
	for i, arg := range call.Args {
		if i > info.FormatStringIndex {
			formatArgs = append(formatArgs, arg)
		}
	}

	// Create tr.T call
	trCall := ft.createTrCall(key, formatArgs)

	// Build new arguments: preserve everything before format string, add "%s", add tr.T call
	newArgs := make([]ast.Expr, 0, info.FormatStringIndex+2)

	// Add arguments before format string (e.g., the error for errors.Wrapf)
	for i := 0; i < info.FormatStringIndex; i++ {
		newArgs = append(newArgs, call.Args[i])
	}

	// Add the "%s" format string and tr.T call
	newArgs = append(newArgs, call.Args[info.FormatStringIndex], trCall)

	call.Args = newArgs
}

// extractFormatArgs extracts the format arguments from a call
func (ft *FormatTransformer) extractFormatArgs(call *ast.CallExpr, info *FormatCallInfo) []ast.Expr {
	var args []ast.Expr
	for i, arg := range call.Args {
		if i > info.FormatStringIndex {
			args = append(args, arg)
		}
	}
	return args
}

// transformRegularFunctionCall handles regular function calls with string literals
func (ft *FormatTransformer) transformRegularFunctionCall(call *ast.CallExpr) {
	// Check each argument for string literals
	for i, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			// Check if this string has an i18n-skip comment
			if ft.skipPositions[lit.Pos()] {
				continue // Skip transformation
			}

			// Check if we have a translation key for this string
			// Note: The string has already been filtered by regex patterns during extraction
			key, exists := ft.stringMap[lit.Value]
			if !exists {
				continue
			}

			// Determine function name
			funcName := ft.getFunctionName(call)

			// Check if we should transform this function based on mode
			switch ft.transformMode {
			case "user-facing", "all-marked":
				// In these modes, check against known user-facing functions
				if !ft.isUserFacingFunction(funcName) {
					continue
				}
			case "with-comments":
				// In with-comments mode, the stringMap already contains only strings with i18n-todo comments
				// So we transform all strings that have keys
			case "all":
				// Transform all functions with string literals
			}

			// Replace the string literal with tr.T call
			call.Args[i] = ft.createTrCall(key, nil)
			ft.transformed = true
			ft.requiredImports["messages"] = true
			ft.requiredImports["i18n"] = true
			// Track the transformed string (unquoted value)
			unquotedStr := strings.Trim(lit.Value, "`\"")
			ft.transformedStrings[unquotedStr] = key

			// Clear the Lparen/Rparen positions to let the formatter recreate them
			call.Lparen = 0
			call.Rparen = 0
		}
	}
}

// isUserFacingFunction checks if a function is known to display user-facing text
func (ft *FormatTransformer) isUserFacingFunction(funcName string) bool {
	// Check regex patterns first if provided
	for _, regex := range ft.userFacingRegexes {
		if regex.MatchString(funcName) {
			return true
		}
	}

	// Check exact matches (package.Function)
	exactMatches := map[string]bool{
		// fmt package - display functions
		"fmt.Print":    true,
		"fmt.Println":  true,
		"fmt.Fprint":   true,
		"fmt.Fprintln": true,
		"fmt.Sprint":   true,
		"fmt.Sprintln": true,

		// log package - logging functions
		"log.Print":   true,
		"log.Println": true,
		"log.Fatal":   true,
		"log.Fatalln": true,
		"log.Panic":   true,
		"log.Panicln": true,

		// errors package
		"errors.New": true,
	}

	if exactMatches[funcName] {
		return true
	}

	// Check method names (anything.MethodName)
	// This handles logger.Info(), slog.Info(), customLogger.Error(), etc.
	parts := strings.Split(funcName, ".")
	if len(parts) == 2 {
		methodName := parts[1]

		// Common logging method names
		loggingMethods := map[string]bool{
			// Logging levels
			"Info":    true,
			"Error":   true,
			"Warn":    true,
			"Warning": true,
			"Debug":   true,
			"Trace":   true,
			"Fatal":   true,
			"Panic":   true,

			// Common logging methods
			"Log":     true,
			"Logf":    true,
			"Print":   true,
			"Println": true,
			"Printf":  true,

			// Message methods (for structured loggers)
			"Msg":  true,
			"Send": true,
		}

		if loggingMethods[methodName] {
			return true
		}

		// Handle chained calls like "chained.Msg"
		if parts[0] == "chained" && loggingMethods[methodName] {
			return true
		}
	}

	// Handle deeper chains like "chained.logger.Info.Msg"
	if len(parts) > 2 && parts[0] == "chained" {
		// Get the last part as the method name
		lastMethod := parts[len(parts)-1]
		loggingMethods := map[string]bool{
			"Info": true, "Error": true, "Warn": true, "Debug": true,
			"Msg": true, "Send": true, "Log": true, "Print": true,
		}
		if loggingMethods[lastMethod] {
			return true
		}
	}

	return false
}

// getFunctionName extracts the full function name from a call expression
func (ft *FormatTransformer) getFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// Build the full selector path recursively
		return ft.buildSelectorPath(fun)
	case *ast.Ident:
		// Simple function name
		return fun.Name
	}
	return ""
}

// buildSelectorPath recursively builds the full path for a selector expression
func (ft *FormatTransformer) buildSelectorPath(sel *ast.SelectorExpr) string {
	switch x := sel.X.(type) {
	case *ast.Ident:
		// Base case: simple identifier
		return x.Name + "." + sel.Sel.Name
	case *ast.SelectorExpr:
		// Recursive case: nested selector
		return ft.buildSelectorPath(x) + "." + sel.Sel.Name
	case *ast.CallExpr:
		// Chained call: extract the function being called
		if callSel, ok := x.Fun.(*ast.SelectorExpr); ok {
			// For chained calls like logger.WithField().Infof()
			// we return "chained.WithField.Infof" to indicate it's a chain
			return "chained." + ft.buildSelectorPath(callSel) + "." + sel.Sel.Name
		}
		// Generic chained call
		return "chained." + sel.Sel.Name
	case *ast.IndexExpr:
		// Handle indexed expressions like arr[0].Method()
		if ident, ok := x.X.(*ast.Ident); ok {
			return ident.Name + "[idx]." + sel.Sel.Name
		}
		return "indexed." + sel.Sel.Name
	default:
		// For any other expression type, just return the method name
		// This handles cases like function returns: GetLogger().Info()
		return sel.Sel.Name
	}
}

// formatNode converts an AST node back to source code
func (ft *FormatTransformer) formatNode(node ast.Node) ([]byte, error) {
	// Before formatting, ensure all comments are properly sorted by position
	if file, ok := node.(*ast.File); ok {
		sortComments(ft.fset, file)
	}

	var buf bytes.Buffer
	err := format.Node(&buf, ft.fset, node)
	if err != nil {
		return nil, err
	}

	// Post-process to fix multiline key issues
	result := buf.Bytes()
	result = ft.fixMultilineKeys(result)

	return result, nil
}

// COMMENTED OUT - No longer needed with new approach
/*
// removeTransformedI18nTodoComments removes i18n-todo comments that appear after transformed strings
func (ft *FormatTransformer) removeTransformedI18nTodoComments(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	var result []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if this line contains both a translator call and an i18n-todo comment
		if strings.Contains(line, ft.trPattern+"(messages.Keys.") && strings.Contains(line, "i18n-todo:") {
			// Find and remove the i18n-todo comment (both block and line styles)

			// Handle block comments
			if idx := strings.Index(line, "/"+"* i18n-todo:"); idx >= 0 {
				// Find the closing part
				if endIdx := strings.Index(line[idx:], "*"+"/"); endIdx >= 0 {
					// Remove the entire comment block
					line = line[:idx] + line[idx+endIdx+2:]
					// Clean up extra spaces
					line = strings.TrimRight(line, " \t")
				}
			}

			// Handle line comments // i18n-todo: ...
			if idx := strings.Index(line, "// i18n-todo:"); idx >= 0 {
				// Keep everything before the comment
				line = strings.TrimRight(line[:idx], " \t")
			}
		}

		// Also check if this is a standalone i18n-todo comment line that should be removed
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "// i18n-todo:") || strings.HasPrefix(trimmed, "/"+"* i18n-todo:") {
			// Check if the previous line was transformed (contains tr.T call)
			if i > 0 && strings.Contains(result[len(result)-1], ft.trPattern+"(messages.Keys.") {
				// Skip this comment line
				continue
			}
		}

		result = append(result, line)
	}

	return []byte(strings.Join(result, "\n"))
}
*/

// fixMultilineKeys fixes issues where message keys are split across lines
func (ft *FormatTransformer) fixMultilineKeys(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	var fixed []string

	// Build a pattern to match the translator call
	// We need to match the pattern with "(messages.Keys." or "(messages."
	trCallPrefix := ft.trPattern + "(messages."

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if this line contains a translator call that might be split
		if strings.Contains(line, trCallPrefix) {
			// Find the complete expression
			combined := line
			j := i
			openParens := strings.Count(line, "(") - strings.Count(line, ")")

			// Keep accumulating lines until we balance parentheses
			for openParens > 0 && j+1 < len(lines) {
				j++
				nextLine := lines[j]
				combined += " " + strings.TrimSpace(nextLine)
				openParens += strings.Count(nextLine, "(") - strings.Count(nextLine, ")")
			}

			// Now we have the complete expression, clean it up
			if j > i {
				// Extract indentation from original line
				indent := ""
				for _, ch := range line {
					if ch == ' ' || ch == '\t' {
						indent += string(ch)
					} else {
						break
					}
				}

				// Clean up the combined line
				combined = ft.cleanupTrCall(combined, indent)
				fixed = append(fixed, combined)
				i = j // Skip the lines we just processed
				continue
			}
		}

		// Normal line
		fixed = append(fixed, line)
	}

	return []byte(strings.Join(fixed, "\n"))
}

// cleanupTrCall cleans up a tr.T call that was spread across multiple lines
func (ft *FormatTransformer) cleanupTrCall(combined string, indent string) string {
	// Remove extra whitespace and newlines within the expression
	cleaned := regexp.MustCompile(`\s+`).ReplaceAllString(combined, " ")

	// Fix spacing around dots in message keys
	cleaned = regexp.MustCompile(`\.\s+`).ReplaceAllString(cleaned, ".")
	cleaned = regexp.MustCompile(`\s+\.`).ReplaceAllString(cleaned, ".")

	// Fix spacing around parentheses
	cleaned = regexp.MustCompile(`\(\s+`).ReplaceAllString(cleaned, "(")
	cleaned = regexp.MustCompile(`\s+\)`).ReplaceAllString(cleaned, ")")

	// Fix spacing around commas
	cleaned = regexp.MustCompile(`\s*,\s*`).ReplaceAllString(cleaned, ", ")

	// The translator pattern is exactly as provided by the user
	// We just need to ensure there's no extra whitespace between the pattern and the opening parenthesis
	patternEscaped := regexp.QuoteMeta(ft.trPattern)
	cleaned = regexp.MustCompile(patternEscaped+`\s*\(`).ReplaceAllString(cleaned, ft.trPattern+"(")

	// Validation: ensure the cleaned line contains the exact pattern we expect
	expectedPattern := ft.trPattern + "(messages.Keys."
	if !strings.Contains(cleaned, expectedPattern) {
		// If our cleanup didn't produce the expected pattern, log a warning
		// This helps us debug issues with complex translator patterns
		fmt.Fprintf(os.Stderr, "Warning: cleaned line doesn't contain expected pattern '%s'\n", expectedPattern)
		fmt.Fprintf(os.Stderr, "Cleaned line: %s\n", cleaned)
	}

	// Trim and add back the indent
	return strings.TrimSpace(cleaned)
}

// sortComments ensures comments are properly sorted by position
func sortComments(fset *token.FileSet, file *ast.File) {
	// Sort comment groups by position to ensure proper ordering
	for i := 0; i < len(file.Comments)-1; i++ {
		for j := i + 1; j < len(file.Comments); j++ {
			if file.Comments[i].Pos() > file.Comments[j].Pos() {
				file.Comments[i], file.Comments[j] = file.Comments[j], file.Comments[i]
			}
		}
	}
}

// findParentNode finds the appropriate parent node for attaching comments
func (ft *FormatTransformer) findParentNode(file *ast.File, target ast.Node) ast.Node {
	var parent ast.Node

	// Walk the AST to find the parent of the target node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// Check children of this node
		var foundTarget bool
		ast.Inspect(n, func(child ast.Node) bool {
			if child == target {
				foundTarget = true
				return false
			}
			return child != n // Don't recurse into self
		})

		if foundTarget {
			// This node is the direct parent of our target
			parent = n
			return false
		}

		return true
	})

	return parent
}

// findParentCall finds the enclosing CallExpr for a node
func (ft *FormatTransformer) findParentCall(file *ast.File, target ast.Node) *ast.CallExpr {
	var parentCall *ast.CallExpr

	// Walk the AST to find the enclosing call expression
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// Check if this is a call expression
		if call, ok := n.(*ast.CallExpr); ok {
			// Check if this call contains our target
			var containsTarget bool
			ast.Inspect(call, func(child ast.Node) bool {
				if child == target {
					containsTarget = true
					return false
				}
				return true
			})

			if containsTarget {
				// This is the call containing our target
				parentCall = call
				return false
			}
		}

		return true
	})

	return parentCall
}

// attachCommentsToNodes ensures comments are properly attached to AST nodes
func (ft *FormatTransformer) attachCommentsToNodes(file *ast.File, nodesToAttachComments map[ast.Node][]*ast.Comment) {
	// The key insight is that Go's AST preserves comments through the file.Comments list
	// and the go/format package uses position information to associate comments with code.
	// We need to ensure that i18n-skip comments remain in the file.Comments list
	// with the correct positions relative to the string literals they mark.

	// Build a set of all comments already in the file
	commentSet := make(map[*ast.Comment]bool)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			commentSet[c] = true
		}
	}

	// For each node that has associated skip comments
	for node, comments := range nodesToAttachComments {
		// Handle different node types to ensure proper comment association
		switch n := node.(type) {
		case *ast.ValueSpec:
			// For const/var declarations, ensure comment stays attached
			if len(comments) > 0 {
				// For ValueSpec, we need to ensure the comment is positioned correctly
				// Check if comment group already exists in the file
				var existingGroup *ast.CommentGroup
				for _, cg := range file.Comments {
					if len(cg.List) > 0 {
						for _, c := range comments {
							for _, existing := range cg.List {
								if existing == c {
									existingGroup = cg
									break
								}
							}
							if existingGroup != nil {
								break
							}
						}
					}
				}

				if existingGroup != nil {
					// Use existing group
					n.Comment = existingGroup
				} else {
					// Create new comment group
					n.Comment = &ast.CommentGroup{List: comments}
				}
			}

		case *ast.GenDecl:
			// For general declarations, find the specific ValueSpec
			for _, spec := range n.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					// Check if any of the values in this spec need the comment
					for _, val := range vs.Values {
						if lit, ok := val.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if ft.skipPositions[lit.Pos()] {
								// This is the ValueSpec that needs the comment
								if len(comments) > 0 {
									// Check if comment group already exists
									var existingGroup *ast.CommentGroup
									for _, cg := range file.Comments {
										if len(cg.List) > 0 && cg.List[0] == comments[0] {
											existingGroup = cg
											break
										}
									}

									if existingGroup == nil {
										// Create new comment group if needed
										existingGroup = &ast.CommentGroup{List: comments}
									}

									vs.Comment = existingGroup
								}
							}
						}
					}
				}
			}

		default:
			// For other node types (AssignStmt, ExprStmt, etc.), we rely on
			// the comment being in the correct position in file.Comments
			// The go/format package will associate them based on position
		}
	}

	// Ensure all skip comments are properly included in the file's comment list
	// This is crucial for preserving comments that couldn't be directly attached
	for _, comments := range nodesToAttachComments {
		for _, comment := range comments {
			if !commentSet[comment] {
				// This comment isn't in the file yet, add it
				found := false
				commentPos := ft.fset.Position(comment.Pos())

				// Try to find an existing comment group on the same line to add to
				for _, cg := range file.Comments {
					if len(cg.List) > 0 {
						firstPos := ft.fset.Position(cg.List[0].Pos())
						if firstPos.Line == commentPos.Line {
							// Add to this existing group
							cg.List = append(cg.List, comment)
							commentSet[comment] = true
							found = true
							break
						}
					}
				}

				if !found {
					// Create a new comment group for this comment
					newGroup := &ast.CommentGroup{List: []*ast.Comment{comment}}

					// Insert in the correct position to maintain order
					inserted := false
					for i, cg := range file.Comments {
						if len(cg.List) > 0 {
							cgPos := ft.fset.Position(cg.List[0].Pos())
							if cgPos.Line > commentPos.Line {
								// Insert before this comment group
								file.Comments = append(file.Comments[:i], append([]*ast.CommentGroup{newGroup}, file.Comments[i:]...)...)
								inserted = true
								break
							}
						}
					}

					if !inserted {
						// Add at the end
						file.Comments = append(file.Comments, newGroup)
					}
					commentSet[comment] = true
				}
			}
		}
	}
}
