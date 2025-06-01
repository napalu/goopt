package ast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

// FormatTransformer handles AST transformation of format function calls
type FormatTransformer struct {
	fset            *token.FileSet
	stringMap       map[string]string  // maps string literals to translation keys
	requiredImports map[string]bool    // tracks imports needed
	transformed     bool               // tracks if any transformations were made
	detector        *FormatDetector    // generic format function detector
	packagePath     string             // path to the messages package
	userFacingOnly  bool               // if true, only transform known user-facing functions
}

// NewFormatTransformer creates a new format transformer
func NewFormatTransformer(stringMap map[string]string) *FormatTransformer {
	return &FormatTransformer{
		fset:            token.NewFileSet(),
		stringMap:       stringMap,
		requiredImports: make(map[string]bool),
		transformed:     false,
		detector:        NewFormatDetector(),
		packagePath:     "./messages", // default value
		userFacingOnly:  true,         // default to only transforming known user-facing functions
	}
}

// SetMessagePackagePath sets the path to the messages package
func (ft *FormatTransformer) SetMessagePackagePath(path string) {
	ft.packagePath = path
}

// SetUserFacingOnly sets whether to only transform known user-facing functions
func (ft *FormatTransformer) SetUserFacingOnly(userFacingOnly bool) {
	ft.userFacingOnly = userFacingOnly
}

// TransformFile transforms format functions in a file
func (ft *FormatTransformer) TransformFile(filename string, src []byte) ([]byte, error) {
	// Parse the file
	file, err := parser.ParseFile(ft.fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Transform the AST
	ast.Inspect(file, ft.transformNode)

	// Add required imports if any transformations were made
	if ft.transformed {
		ft.addImports(file)
	}

	// Convert back to source
	return formatNode(ft.fset, file)
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

// FunctionInfo holds information about a function call
type FunctionInfo struct {
	pkg      string // package name (e.g., "fmt", "log")
	receiver string // receiver for method calls (e.g., "logger")
	function string // function name (e.g., "Printf") 
	funcType string // normalized function type for handling
}

// identifyFunction analyzes a CallExpr to identify the function being called
func (ft *FormatTransformer) identifyFunction(call *ast.CallExpr) *FunctionInfo {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// Could be pkg.Func or receiver.Method
		switch x := fun.X.(type) {
		case *ast.Ident:
			// Simple case: fmt.Printf or logger.Printf
			return &FunctionInfo{
				pkg:      x.Name,
				function: fun.Sel.Name,
				funcType: ft.getFuncType(x.Name, fun.Sel.Name),
			}
		case *ast.CallExpr:
			// Chained call: log.Info().Msgf
			if chainInfo := ft.getChainedCallInfo(x, fun.Sel.Name); chainInfo != nil {
				return chainInfo
			}
		case *ast.SelectorExpr:
			// Deeper chain: log.WithField().Infof
			if chainInfo := ft.getDeepChainInfo(x, fun.Sel.Name); chainInfo != nil {
				return chainInfo
			}
		}
	}
	return nil
}

// getFuncType returns a normalized function type for transformation
func (ft *FormatTransformer) getFuncType(pkg, function string) string {
	// Standard library format functions
	if pkg == "fmt" {
		switch function {
		case "Printf":
			return "printf"
		case "Sprintf":
			return "sprintf"
		case "Fprintf":
			return "fprintf"
		case "Errorf":
			return "errorf"
		}
	}
	
	if pkg == "log" {
		switch function {
		case "Printf", "Fatalf", "Panicf":
			return "printf"
		}
	}

	// Common logging libraries
	switch function {
	case "Msgf":
		return "msgf"
	case "Infof", "Debugf", "Warnf", "Errorf":
		return "infof" // logrus style
	}

	return ""
}

// getChainedCallInfo handles chained method calls like log.Info().Msgf()
func (ft *FormatTransformer) getChainedCallInfo(chainCall *ast.CallExpr, methodName string) *FunctionInfo {
	if methodName == "Msgf" {
		// This is likely a zerolog-style call
		return &FunctionInfo{
			funcType: "msgf",
			function: methodName,
		}
	}
	return nil
}

// getDeepChainInfo handles deeper chains like log.WithFields().Infof()
func (ft *FormatTransformer) getDeepChainInfo(sel *ast.SelectorExpr, methodName string) *FunctionInfo {
	switch methodName {
	case "Infof", "Debugf", "Warnf", "Errorf":
		return &FunctionInfo{
			funcType: "infof",
			function: methodName,
		}
	}
	return nil
}

// isHandledFormatFunction checks if we handle this function type
func (ft *FormatTransformer) isHandledFormatFunction(info *FunctionInfo) bool {
	return info.funcType != ""
}

// transformPrintf transforms Printf-style calls
func (ft *FormatTransformer) transformPrintf(call *ast.CallExpr, info *FunctionInfo, key string) {
	// Change Printf to Print
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		newName := strings.TrimSuffix(sel.Sel.Name, "f")
		sel.Sel = ast.NewIdent(newName)
	}

	// Create tr.T call with all arguments
	trCall := ft.createTrCall(key, call.Args[1:])
	
	// Replace arguments with just the tr.T call
	call.Args = []ast.Expr{trCall}
}

// transformSprintf transforms Sprintf calls to direct tr.T calls
func (ft *FormatTransformer) transformSprintf(call *ast.CallExpr, info *FunctionInfo, key string) {
	// Replace the entire call with tr.T
	call.Fun = &ast.SelectorExpr{
		X:   ast.NewIdent("tr"),
		Sel: ast.NewIdent("T"),
	}

	// Replace arguments
	call.Args = ft.createTrCallArgs(key, call.Args[1:])
}

// transformFprintf transforms Fprintf calls
func (ft *FormatTransformer) transformFprintf(call *ast.CallExpr, info *FunctionInfo, key string) {
	// Change Fprintf to Fprint
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		sel.Sel = ast.NewIdent("Fprint")
	}

	// Keep the writer as first arg, add tr.T call as second
	writer := call.Args[0]
	trCall := ft.createTrCall(key, call.Args[2:]) // Skip writer and format string
	call.Args = []ast.Expr{writer, trCall}
}

// transformErrorf transforms Errorf calls with special handling for %w
func (ft *FormatTransformer) transformErrorf(call *ast.CallExpr, info *FunctionInfo, key string, formatStr string) {
	// Check if format string contains %w for error wrapping
	unquoted := strings.Trim(formatStr, `"`)
	hasErrorWrap := strings.Contains(unquoted, "%w")

	if hasErrorWrap {
		// Need to preserve error wrapping
		// fmt.Errorf("msg: %w", err) -> fmt.Errorf("%s: %w", tr.T(key), err)
		
		// Find the position of %w and extract non-error format args
		var nonErrorArgs []ast.Expr
		errorArg := call.Args[len(call.Args)-1] // Assume %w is last
		
		if len(call.Args) > 2 {
			nonErrorArgs = call.Args[1 : len(call.Args)-1]
		}

		// Create new format string
		call.Args[0] = &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"%s: %w"`,
		}

		// Create tr.T call for the message part
		trCall := ft.createTrCall(key, nonErrorArgs)
		
		// New args: format, tr.T result, error
		call.Args = []ast.Expr{call.Args[0], trCall, errorArg}
	} else {
		// No error wrapping, convert to errors.New
		call.Fun = &ast.SelectorExpr{
			X:   ast.NewIdent("errors"),
			Sel: ast.NewIdent("New"),
		}

		// Create tr.T call with all format args
		trCall := ft.createTrCall(key, call.Args[1:])
		call.Args = []ast.Expr{trCall}
		
		ft.requiredImports["errors"] = true
	}
}

// transformMsgf transforms zerolog-style Msgf calls
func (ft *FormatTransformer) transformMsgf(call *ast.CallExpr, info *FunctionInfo, key string) {
	// Change Msgf to Msg
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		sel.Sel = ast.NewIdent("Msg")
	}

	// Create tr.T call
	trCall := ft.createTrCall(key, call.Args[1:])
	call.Args = []ast.Expr{trCall}
}

// transformLogrusStyle transforms logrus-style format methods
func (ft *FormatTransformer) transformLogrusStyle(call *ast.CallExpr, info *FunctionInfo, key string) {
	// Change Infof to Info, etc.
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		newName := strings.TrimSuffix(sel.Sel.Name, "f")
		sel.Sel = ast.NewIdent(newName)
	}

	// Create tr.T call
	trCall := ft.createTrCall(key, call.Args[1:])
	call.Args = []ast.Expr{trCall}
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
			Tok: token.IMPORT,
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
	
	// Insert after imports
	if importIndex >= 0 {
		newDecls := make([]ast.Decl, 0, len(file.Decls)+1)
		newDecls = append(newDecls, file.Decls[:importIndex+1]...)
		newDecls = append(newDecls, trDecl)
		newDecls = append(newDecls, file.Decls[importIndex+1:]...)
		file.Decls = newDecls
	}
}

// extractFormatString attempts to extract a format string from an expression
// Returns the string and whether it's a literal (or concatenation of literals)
func (ft *FormatTransformer) extractFormatString(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			// Remove quotes
			return strings.Trim(e.Value, "`\""), true
		}
	case *ast.BinaryExpr:
		// Handle string concatenation
		if e.Op == token.ADD {
			left, leftOk := ft.extractFormatString(e.X)
			right, rightOk := ft.extractFormatString(e.Y)
			if leftOk && rightOk {
				return left + right, true
			}
		}
	}
	return "", false
}

// Generic transformation methods that work with FormatCallInfo

// transformGenericPrintf handles Printf-style functions generically
func (ft *FormatTransformer) transformGenericPrintf(call *ast.CallExpr, info *FormatCallInfo, key string) {
	// Remove 'f' from function name if it ends with 'f'
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		funcName := sel.Sel.Name
		if strings.HasSuffix(funcName, "f") {
			sel.Sel = ast.NewIdent(strings.TrimSuffix(funcName, "f"))
		}
	}

	// Create tr.T call with format arguments
	args := ft.extractFormatArgs(call, info)
	trCall := ft.createTrCall(key, args)
	
	// Replace all arguments with just the tr.T call
	call.Args = []ast.Expr{trCall}
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
		call.Args[info.FormatStringIndex] = &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"%s: %w"`,
		}
		
		// Extract non-error format args
		var formatArgs []ast.Expr
		var errorArg ast.Expr
		
		for i, arg := range call.Args {
			if i > info.FormatStringIndex {
				if i == len(call.Args)-1 && hasErrorWrap {
					errorArg = arg
				} else {
					formatArgs = append(formatArgs, arg)
				}
			}
		}
		
		// Create tr.T call
		trCall := ft.createTrCall(key, formatArgs)
		
		// New args
		newArgs := []ast.Expr{call.Args[info.FormatStringIndex], trCall}
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
			
			// Check if we should transform this function
			if ft.userFacingOnly {
				// In user-facing only mode, check against known user-facing functions
				if !ft.isUserFacingFunction(funcName) {
					continue
				}
			}
			// If not in user-facing only mode, transform all functions with string literals
			
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
	// Check exact matches first (package.Function)
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
	}
	
	return false
}

// getFunctionName extracts the full function name from a call expression
func (ft *FormatTransformer) getFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// pkg.Function or receiver.Method
		if ident, ok := fun.X.(*ast.Ident); ok {
			return ident.Name + "." + fun.Sel.Name
		}
	case *ast.Ident:
		// Simple function name
		return fun.Name
	}
	return ""
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