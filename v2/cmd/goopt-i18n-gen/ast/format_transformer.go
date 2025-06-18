package ast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// FormatTransformer handles AST transformation of format function calls
type FormatTransformer struct {
	fset              *token.FileSet
	stringMap         map[string]string    // maps string literals to translation keys
	requiredImports   map[string]bool      // tracks imports needed
	transformed       bool                 // tracks if any transformations were made
	detector          *FormatDetector      // generic format function detector
	packagePath       string               // path to the messages package
	transformMode     string               // "user-facing", "with-comments", "all-marked", "all"
	i18nTodoMap       map[token.Pos]string // maps string literal positions to i18n-todo message keys
	userFacingRegexes []*regexp.Regexp     // regex patterns to identify user-facing functions
}

// NewFormatTransformer creates a new format transformer
func NewFormatTransformer(stringMap map[string]string) *FormatTransformer {
	return &FormatTransformer{
		fset:            token.NewFileSet(),
		stringMap:       stringMap,
		requiredImports: make(map[string]bool),
		transformed:     false,
		detector:        NewFormatDetector(),
		packagePath:     "messages",    // default value (will be resolved to full module path)
		transformMode:   "user-facing", // default to only transforming known user-facing functions
		i18nTodoMap:     make(map[token.Pos]string),
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

// buildI18nTodoMap scans the AST to find string literals with i18n-todo comments
func (ft *FormatTransformer) buildI18nTodoMap(file *ast.File) {
	// Clear the map
	ft.i18nTodoMap = make(map[token.Pos]string)

	// First, collect all i18n-todo comments and their positions
	todoComments := make(map[token.Pos]string)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			// Check if this is an i18n-todo comment
			if strings.Contains(c.Text, "i18n-todo:") {
				// Extract the message key from the comment
				// Format: // i18n-todo: tr.T(messages.Keys.Hello)
				parts := strings.Split(c.Text, "i18n-todo:")
				if len(parts) < 2 {
					continue
				}

				todoText := strings.TrimSpace(parts[1])
				// Extract the key from tr.T(messages.Keys.XXX) or similar patterns
				if keyStart := strings.Index(todoText, "("); keyStart != -1 {
					if keyEnd := strings.LastIndex(todoText, ")"); keyEnd != -1 {
						keyExpr := todoText[keyStart+1 : keyEnd]
						// Remove any quotes if present
						keyExpr = strings.Trim(keyExpr, `"'`)
						// Store by comment position
						todoComments[c.Pos()] = keyExpr
					}
				}
			}
		}
	}

	// Now walk the AST to find string literals that are on the same line as i18n-todo comments
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			litPos := ft.fset.Position(lit.Pos())

			// Check if there's an i18n-todo comment on the same line
			for commentPos, keyExpr := range todoComments {
				cPos := ft.fset.Position(commentPos)
				if cPos.Line == litPos.Line {
					// Found a match - map the string literal position to the key
					ft.i18nTodoMap[lit.Pos()] = keyExpr
					break
				}
			}
		}
		return true
	})
}

// TransformFile transforms format functions in a file
func (ft *FormatTransformer) TransformFile(filename string, src []byte) ([]byte, error) {
	// Parse the file
	file, err := parser.ParseFile(ft.fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Build i18n-todo map if needed for certain transform modes
	if ft.transformMode == "with-comments" || ft.transformMode == "all-marked" {
		ft.buildI18nTodoMap(file)
	}

	// Transform the AST based on mode
	switch ft.transformMode {
	case "user-facing":
		// Only transform user-facing functions
		ast.Inspect(file, ft.transformNode)
	case "with-comments":
		// Only transform strings with i18n-todo comments
		if len(ft.i18nTodoMap) > 0 {
			ft.applyI18nTodoTransformations(file)
		}
	case "all-marked":
		// Transform both user-facing AND i18n-todo comments
		ast.Inspect(file, ft.transformNode)
		if len(ft.i18nTodoMap) > 0 {
			ft.applyI18nTodoTransformations(file)
		}
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
	return formatNode(ft.fset, file)
}

// transformAllStrings transforms all string literals that have translation keys
func (ft *FormatTransformer) transformAllStrings(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			// First handle format functions as usual
			ft.transformNode(x)
		case *ast.AssignStmt:
			// Handle assignments
			for i, rhs := range x.Rhs {
				if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					for quotedStr, key := range ft.stringMap {
						if lit.Value == quotedStr {
							trCall := ft.createTrCall(key, nil)
							if trCall != nil {
								x.Rhs[i] = trCall
								ft.transformed = true
								ft.requiredImports["messages"] = true
								ft.requiredImports["i18n"] = true
							}
							break
						}
					}
				}
			}
		case *ast.ValueSpec:
			// Handle var/const declarations
			for i, val := range x.Values {
				if lit, ok := val.(*ast.BasicLit); ok && lit.Kind == token.STRING {
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
		}
		return true
	})
}

// applyI18nTodoTransformations applies transformations based on i18n-todo comments
func (ft *FormatTransformer) applyI18nTodoTransformations(file *ast.File) {
	// Walk the AST and transform string literals that have i18n-todo comments
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			// Handle function calls with string literals
			for i, arg := range x.Args {
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if _, found := ft.i18nTodoMap[lit.Pos()]; found {
						// Check if we have a key for this string
						unquotedStr := strings.Trim(lit.Value, `"'`+"`")
						for quotedStr, key := range ft.stringMap {
							if strings.Trim(quotedStr, `"'`+"`") == unquotedStr {
								// Found a match - create tr.T call
								trCall := ft.createTrCall(key, nil)
								if trCall != nil {
									x.Args[i] = trCall
									ft.transformed = true
									ft.requiredImports["messages"] = true
									ft.requiredImports["i18n"] = true
								}
								break
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			// Handle assignments like: msg := "World" // i18n-todo: tr.T(messages.Keys.World)
			for i, rhs := range x.Rhs {
				if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if _, found := ft.i18nTodoMap[lit.Pos()]; found {
						// Check if the key exists in our string map
						// We need to check if we have a transformation for this string
						unquotedStr := strings.Trim(lit.Value, `"'`+"`")
						for quotedStr, key := range ft.stringMap {
							if strings.Trim(quotedStr, `"'`+"`") == unquotedStr {
								// Found a match - create tr.T call
								trCall := ft.createTrCall(key, nil)
								if trCall != nil {
									x.Rhs[i] = trCall
									ft.transformed = true
									ft.requiredImports["messages"] = true
									ft.requiredImports["i18n"] = true
								}
								break
							}
						}
					}
				}
			}
		case *ast.ValueSpec:
			// Handle var/const declarations
			for i, val := range x.Values {
				if lit, ok := val.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if _, found := ft.i18nTodoMap[lit.Pos()]; found {
						unquotedStr := strings.Trim(lit.Value, `"'`+"`")
						for quotedStr, key := range ft.stringMap {
							if strings.Trim(quotedStr, `"'`+"`") == unquotedStr {
								// Found a match - create tr.T call
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
		return true
	}

	// If not a format function, check for regular function calls with string literals
	ft.transformRegularFunctionCall(call)

	return true
}

// createTrCall creates a tr.T function call expression
func (ft *FormatTransformer) createTrCall(key string, args []ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("tr"),
			Sel: ast.NewIdent("T"),
		},
		Args: ft.createTrCallArgs(key, args),
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
	parts := strings.Split(key, ".")

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

// addImports adds required imports to the file
func (ft *FormatTransformer) addImports(file *ast.File) {
	// Check which imports we need to add
	needMessages := ft.requiredImports["messages"]
	needErrors := ft.requiredImports["errors"]
	needI18n := needMessages // If we need messages, we need i18n for tr

	if !needMessages && !needErrors {
		return
	}

	// Find or create import declaration
	var importDecl *ast.GenDecl
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			importDecl = genDecl
			break
		}
	}

	// If no import declaration exists, create one after package declaration
	if importDecl == nil {
		importDecl = &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{},
		}

		// Insert after package declaration
		newDecls := make([]ast.Decl, 0, len(file.Decls)+1)
		newDecls = append(newDecls, file.Decls[0]) // package decl
		newDecls = append(newDecls, importDecl)
		newDecls = append(newDecls, file.Decls[1:]...)
		file.Decls = newDecls
	}

	// Add required imports if not already present
	if needMessages && !ft.hasImport(importDecl, "messages") {
		importDecl.Specs = append(importDecl.Specs, &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf(`"%s"`, ft.packagePath),
			},
		})
	}

	if needErrors && !ft.hasImport(importDecl, "errors") {
		importDecl.Specs = append(importDecl.Specs, &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: `"errors"`,
			},
		})
	}

	// We also need to ensure tr is available
	if needI18n && !ft.hasImport(importDecl, "github.com/napalu/goopt/v2/i18n") {
		importDecl.Specs = append(importDecl.Specs, &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: `"github.com/napalu/goopt/v2/i18n"`,
			},
		})
	}

	// Add tr variable initialization if needed
	if needMessages {
		ft.addTrInitialization(file)
	}
}

// hasImport checks if an import already exists
func (ft *FormatTransformer) hasImport(importDecl *ast.GenDecl, pkg string) bool {
	for _, spec := range importDecl.Specs {
		if importSpec, ok := spec.(*ast.ImportSpec); ok {
			path := strings.Trim(importSpec.Path.Value, `"`)
			if strings.HasSuffix(path, "/"+pkg) || path == pkg {
				return true
			}
		}
	}
	return false
}

// addTrInitialization adds tr variable declaration/initialization
func (ft *FormatTransformer) addTrInitialization(file *ast.File) {
	// Check if tr is already declared
	hasTr := false
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range valueSpec.Names {
						if name.Name == "tr" {
							hasTr = true
							break
						}
					}
				}
			}
		}
	}

	if hasTr {
		return
	}

	// Create tr variable declaration
	trDecl := &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent("tr")},
				Type: &ast.SelectorExpr{
					X:   ast.NewIdent("i18n"),
					Sel: ast.NewIdent("Translator"),
				},
				Comment: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: "// TODO: Initialize with your translator instance"},
					},
				},
			},
		},
	}

	// Find position after imports
	importIndex := -1
	for i, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			importIndex = i
		}
	}

	// Insert after imports, but be careful about //go: directives
	if importIndex >= 0 {
		insertPos := importIndex + 1

		// Skip past any declarations that have //go: directives
		// These directives must be immediately followed by their target declaration
		for insertPos < len(file.Decls) {
			// Check if this declaration has any //go: directive comments
			if genDecl, ok := file.Decls[insertPos].(*ast.GenDecl); ok {
				hasGoDirective := false

				// Check the declaration's doc comments for //go: directives
				if genDecl.Doc != nil {
					for _, comment := range genDecl.Doc.List {
						if strings.HasPrefix(strings.TrimSpace(comment.Text), "//go:") {
							hasGoDirective = true
							break
						}
					}
				}

				// If this declaration has a //go: directive, skip it
				if hasGoDirective {
					insertPos++
					continue
				}
			}
			break
		}

		// Now insert the tr declaration at the safe position
		newDecls := make([]ast.Decl, 0, len(file.Decls)+1)
		newDecls = append(newDecls, file.Decls[:insertPos]...)
		newDecls = append(newDecls, trDecl)
		newDecls = append(newDecls, file.Decls[insertPos:]...)
		file.Decls = newDecls
	}
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
				// Skip - only i18n-todo comments are transformed in this mode
				continue
			case "all":
				// Transform all functions with string literals
			}

			// Replace the string literal with tr.T call
			call.Args[i] = ft.createTrCall(key, nil)
			ft.transformed = true
			ft.requiredImports["messages"] = true
			ft.requiredImports["i18n"] = true
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
func formatNode(fset *token.FileSet, node ast.Node) ([]byte, error) {
	var buf bytes.Buffer
	err := format.Node(&buf, fset, node)
	if err != nil {
		return nil, err
	}

	// Post-process to fix multiline key issues
	result := buf.Bytes()
	result = fixMultilineKeys(result)

	return result, nil
}

// fixMultilineKeys fixes issues where message keys are split across lines
func fixMultilineKeys(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	var fixed []string
	var inMessageKey bool
	var keyBuffer []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check if we're starting a messages.Keys reference
		if strings.Contains(line, "messages.Keys.") && !strings.Contains(line, ")") {
			inMessageKey = true
			keyBuffer = []string{line}
			continue
		}

		// If we're in a message key, accumulate lines
		if inMessageKey {
			// Check if this line ends the key
			if strings.Contains(trimmed, ")") || strings.Contains(trimmed, ",") {
				// Combine all parts on one line
				keyParts := append(keyBuffer, trimmed)
				combined := strings.Join(keyParts, "")
				// Clean up extra whitespace
				combined = strings.ReplaceAll(combined, "\t", " ")
				combined = strings.ReplaceAll(combined, "  ", " ")
				fixed = append(fixed, combined)
				inMessageKey = false
				keyBuffer = nil
			} else {
				// Continue accumulating
				keyBuffer = append(keyBuffer, trimmed)
			}
			continue
		}

		// Normal line
		fixed = append(fixed, line)
	}

	// Handle any remaining buffer
	if len(keyBuffer) > 0 {
		fixed = append(fixed, strings.Join(keyBuffer, ""))
	}

	return []byte(strings.Join(fixed, "\n"))
}
