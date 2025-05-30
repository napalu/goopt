package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/napalu/goopt/v2/i18n"
)

// StringLocation tracks where a string was found
type StringLocation struct {
	File     string
	Line     int
	Function string
	Context  string // Additional context (e.g., "format_function")
}

// ExtractedString represents a string found in the code
type ExtractedString struct {
	Value          string
	Locations      []StringLocation
	IsFormatString bool   // Whether this string is from a format function
}

// StringExtractor extracts string literals from Go source files
type StringExtractor struct {
	tr         i18n.Translator
	matchOnly  *regexp.Regexp
	skipMatch  *regexp.Regexp
	minLength  int
	strings    map[string]*ExtractedString
	currentPkg string
	detector   *FormatDetector // Use generic format detector
}

// NewStringExtractor creates a new string extractor
func NewStringExtractor(tr i18n.Translator, matchOnly, skipMatch string, minLength int) (*StringExtractor, error) {
	var matchRe, skipRe *regexp.Regexp
	var err error

	if matchOnly != "" {
		matchRe, err = regexp.Compile(matchOnly)
		if err != nil {
			return nil, err
		}
	}

	if skipMatch != "" {
		skipRe, err = regexp.Compile(skipMatch)
		if err != nil {
			return nil, err
		}
	}

	return &StringExtractor{
		tr:        tr,
		matchOnly: matchRe,
		skipMatch: skipRe,
		minLength: minLength,
		strings:   make(map[string]*ExtractedString),
		detector:  NewFormatDetector(),
	}, nil
}

// ExtractFromFiles extracts strings from the given files
func (se *StringExtractor) ExtractFromFiles(pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, file := range matches {
		if err := se.extractFromFile(file); err != nil {
			return err
		}
	}

	return nil
}

// ExtractFromString extracts strings from Go source code string
func (se *StringExtractor) ExtractFromString(filename, source string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		return err
	}

	se.currentPkg = node.Name.Name

	// Visit all nodes in the AST
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Build function name including receiver if it's a method
			funcName := x.Name.Name
			if x.Recv != nil && len(x.Recv.List) > 0 {
				// Get receiver type
				recvType := ""
				if starExpr, ok := x.Recv.List[0].Type.(*ast.StarExpr); ok {
					// Pointer receiver
					if ident, ok := starExpr.X.(*ast.Ident); ok {
						recvType = "*" + ident.Name
					}
				} else if ident, ok := x.Recv.List[0].Type.(*ast.Ident); ok {
					// Value receiver
					recvType = ident.Name
				}
				if recvType != "" {
					funcName = recvType + "." + funcName
				}
			}
			se.extractFromFunction(fset, filename, funcName, x.Body)
			return false // Don't descend further, we'll handle the body
		case *ast.FuncLit:
			// Anonymous function
			se.extractFromFunction(fset, filename, "anonymous", x.Body)
			return false
		case *ast.GenDecl:
			// Skip const and var declarations at package level
			if x.Tok == token.CONST || x.Tok == token.VAR {
				return false
			}
		}
		return true
	})

	return nil
}

// extractFromFile extracts strings from a single file
func (se *StringExtractor) extractFromFile(filename string) error {
	// Skip generated files
	if se.isGeneratedFile(filename) {
		return nil
	}

	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	se.currentPkg = node.Name.Name

	// Visit all nodes in the AST
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Build function name including receiver if it's a method
			funcName := x.Name.Name
			if x.Recv != nil && len(x.Recv.List) > 0 {
				// Get receiver type
				recvType := ""
				if starExpr, ok := x.Recv.List[0].Type.(*ast.StarExpr); ok {
					// Pointer receiver
					if ident, ok := starExpr.X.(*ast.Ident); ok {
						recvType = "*" + ident.Name
					}
				} else if ident, ok := x.Recv.List[0].Type.(*ast.Ident); ok {
					// Value receiver
					recvType = ident.Name
				}
				if recvType != "" {
					funcName = recvType + "." + funcName
				}
			}
			se.extractFromFunction(fset, filename, funcName, x.Body)
			return false // Don't descend further, we'll handle the body
		case *ast.FuncLit:
			// Anonymous function
			se.extractFromFunction(fset, filename, "anonymous", x.Body)
			return false
		case *ast.GenDecl:
			// Skip const and var declarations at package level
			if x.Tok == token.CONST || x.Tok == token.VAR {
				return false
			}
		}
		return true
	})

	return nil
}

// extractFromFunction extracts strings from a function body
func (se *StringExtractor) extractFromFunction(fset *token.FileSet, filename, funcName string, body *ast.BlockStmt) {
	if body == nil {
		return
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			// Use generic detector to check for format functions
			if formatInfo := se.detector.DetectFormatCall(node); formatInfo != nil {
				// Extract the format string
				if se.shouldExtract(formatInfo.FormatString) {
					// Get location info
					pos := fset.Position(node.Pos())
					location := StringLocation{
						File:     filename,
						Line:     pos.Line,
						Function: funcName,
						Context:  "format_function",
					}
					
					// Add to our collection
					if existing, ok := se.strings[formatInfo.FormatString]; ok {
						existing.Locations = append(existing.Locations, location)
						existing.IsFormatString = true
					} else {
						se.strings[formatInfo.FormatString] = &ExtractedString{
							Value:          formatInfo.FormatString,
							Locations:      []StringLocation{location},
							IsFormatString: true,
						}
					}
					
					// Don't process children since we handled the format string
					return false
				}
			}
		case *ast.BasicLit:
			if node.Kind == token.STRING {
				// Get the actual string value (remove quotes)
				value := strings.Trim(node.Value, "`\"")
				
				// Apply filters
				if !se.shouldExtract(value) {
					return true
				}

				// Get location info
				pos := fset.Position(node.Pos())
				location := StringLocation{
					File:     filename,
					Line:     pos.Line,
					Function: funcName,
				}

				// Add to our collection
				if existing, ok := se.strings[value]; ok {
					existing.Locations = append(existing.Locations, location)
				} else {
					se.strings[value] = &ExtractedString{
						Value:     value,
						Locations: []StringLocation{location},
					}
				}
			}
		}
		return true
	})
}

// shouldExtract determines if a string should be extracted
func (se *StringExtractor) shouldExtract(value string) bool {
	// Check minimum length
	if len(value) < se.minLength {
		return false
	}

	// Skip if matches exclusion pattern
	if se.skipMatch != nil && se.skipMatch.MatchString(value) {
		return false
	}

	// If inclusion pattern is set, must match it
	if se.matchOnly != nil && !se.matchOnly.MatchString(value) {
		return false
	}

	return true
}

// isGeneratedFile checks if a file is generated
func (se *StringExtractor) isGeneratedFile(filename string) bool {
	// Check common patterns
	if strings.HasSuffix(filename, "_generated.go") {
		return true
	}
	if strings.Contains(filename, "/messages/") && strings.HasSuffix(filename, ".go") {
		return true
	}

	// Check file content for generated header
	content, err := os.ReadFile(filename)
	if err != nil {
		return false
	}

	// Look for generated code marker in first 1KB
	header := string(content[:min(len(content), 1024)])
	return strings.Contains(header, "Code generated") || 
		strings.Contains(header, "DO NOT EDIT")
}

// GetExtractedStrings returns all extracted strings
func (se *StringExtractor) GetExtractedStrings() map[string]*ExtractedString {
	return se.strings
}

// getFunctionName extracts the function name from a call expression
func (se *StringExtractor) getFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// pkg.Function or receiver.Method
		if ident, ok := fun.X.(*ast.Ident); ok {
			return ident.Name + "." + fun.Sel.Name
		}
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

// isFormatFunctionCall checks if a call expression is a format function
func (se *StringExtractor) isFormatFunctionCall(call *ast.CallExpr) bool {
	funcName := se.getFunctionName(call)
	
	// Check if it's a known format function
	formatFunctions := map[string]bool{
		"fmt.Printf":   true,
		"fmt.Sprintf":  true,
		"fmt.Fprintf":  true,
		"fmt.Errorf":   true,
		"log.Printf":   true,
		"log.Fatalf":   true,
		"log.Panicf":   true,
	}
	
	return formatFunctions[funcName]
}

// extractFromFormatCall tries to extract a concatenated string from a format function call
func (se *StringExtractor) extractFromFormatCall(fset *token.FileSet, filename, funcName string, call *ast.CallExpr) bool {
	// Determine which argument contains the format string
	funcInfo := se.getFunctionName(call)
	argIndex := 0
	minArgs := 1
	
	if strings.HasSuffix(funcInfo, ".Fprintf") {
		argIndex = 1  // Fprintf has writer as first arg
		minArgs = 2
	}
	
	if len(call.Args) < minArgs {
		return false
	}
	
	// Try to extract the format string (handling concatenation)
	formatStr, ok := se.extractConcatenatedString(call.Args[argIndex])
	if !ok || formatStr == "" {
		return false
	}
	
	// Apply filters
	if !se.shouldExtract(formatStr) {
		return false
	}
	
	// Get location info
	pos := fset.Position(call.Pos())
	location := StringLocation{
		File:     filename,
		Line:     pos.Line,
		Function: funcName,
		Context:  "format_function", // Mark this as from a format function
	}
	
	// Add to our collection
	if existing, ok := se.strings[formatStr]; ok {
		existing.Locations = append(existing.Locations, location)
		existing.IsFormatString = true
	} else {
		se.strings[formatStr] = &ExtractedString{
			Value:          formatStr,
			Locations:      []StringLocation{location},
			IsFormatString: true,
		}
	}
	
	return true
}

// extractConcatenatedString attempts to extract a full string from concatenated literals
func (se *StringExtractor) extractConcatenatedString(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			// Remove quotes
			return strings.Trim(e.Value, "`\""), true
		}
	case *ast.BinaryExpr:
		// Handle string concatenation
		if e.Op == token.ADD {
			left, leftOk := se.extractConcatenatedString(e.X)
			right, rightOk := se.extractConcatenatedString(e.Y)
			if leftOk && rightOk {
				return left + right, true
			}
		}
	}
	return "", false
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}