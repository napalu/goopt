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
	IsFormatString bool // Whether this string is from a format function
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

	skipLines := se.findSkipCommentLines(fset, node)

	// Visit all nodes in the AST
	ast.Inspect(node, func(n ast.Node) bool {
		// Check if this node should be skipped
		if se.shouldSkipNode(fset, n, skipLines) {
			return false // Don't descend into this node
		}

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
			se.extractFromFunction(fset, filename, funcName, x.Body, skipLines)
			return false // Don't descend further, we'll handle the body
		case *ast.FuncLit:
			// Anonymous function
			se.extractFromFunction(fset, filename, "anonymous", x.Body, skipLines)
			return false
		case *ast.GenDecl:
			// Process var declarations to find strings in errors.New() and similar calls
			if x.Tok == token.VAR {
				for _, spec := range x.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, value := range valueSpec.Values {
							// Process each value to find string literals
							se.extractFromNode(fset, filename, "package-level var", value, skipLines)
						}
					}
				}
				return false
			}
			// Skip const declarations
			if x.Tok == token.CONST {
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

	skipLines := se.findSkipCommentLines(fset, node)

	// Visit all nodes in the AST
	ast.Inspect(node, func(n ast.Node) bool {
		// Check if this node should be skipped
		if se.shouldSkipNode(fset, n, skipLines) {
			return false // Don't descend into this node
		}

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
			se.extractFromFunction(fset, filename, funcName, x.Body, skipLines)
			return false // Don't descend further, we'll handle the body
		case *ast.FuncLit:
			// Anonymous function
			se.extractFromFunction(fset, filename, "anonymous", x.Body, skipLines)
			return false
		case *ast.GenDecl:
			// Process var declarations to find strings in errors.New() and similar calls
			if x.Tok == token.VAR {
				for _, spec := range x.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, value := range valueSpec.Values {
							// Process each value to find string literals
							se.extractFromNode(fset, filename, "package-level var", value, skipLines)
						}
					}
				}
				return false
			}
			// Skip const declarations
			if x.Tok == token.CONST {
				return false
			}
		}
		return true
	})

	return nil
}

// extractFromFunction extracts strings from a function body
func (se *StringExtractor) extractFromFunction(fset *token.FileSet, filename, funcName string, body *ast.BlockStmt, skipLines map[int]bool) {
	if body == nil {
		return
	}

	ast.Inspect(body, func(n ast.Node) bool {
		// Check if this node should be skipped
		if se.shouldSkipNode(fset, n, skipLines) {
			return false
		}

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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (se *StringExtractor) findSkipCommentLines(fset *token.FileSet, file *ast.File) map[int]bool {
	skipLines := make(map[int]bool)

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(strings.ToLower(c.Text), "i18n-skip") {
				pos := fset.Position(c.Pos())
				skipLines[pos.Line] = true

				// Check if this is a standalone comment (comment before pattern)
				// by seeing if there's likely no code on the same line
				// We do this by checking if the comment starts at column 1-10 (allowing for indentation)
				if pos.Column <= 10 || strings.HasPrefix(c.Text, "/*") {
					// This is likely a standalone comment, so skip the next line too
					skipLines[pos.Line+1] = true
				}
			}
		}
	}

	return skipLines
}

// shouldSkipNode checks if a node should be skipped based on comments
func (se *StringExtractor) shouldSkipNode(fset *token.FileSet, node ast.Node, skipLines map[int]bool) bool {
	if node == nil {
		return false
	}

	pos := fset.Position(node.Pos())

	// Check if this line has a skip comment (inline or from previous line)
	if skipLines[pos.Line] {
		return true
	}

	// For string literals, also check the end position for inline comments
	if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		endPos := fset.Position(lit.End())
		if skipLines[endPos.Line] {
			return true
		}
	}

	return false
}

// extractFromNode extracts strings from any AST node (used for var declarations)
func (se *StringExtractor) extractFromNode(fset *token.FileSet, filename, context string, node ast.Node, skipLines map[int]bool) {
	if node == nil {
		return
	}

	ast.Inspect(node, func(n ast.Node) bool {
		// Check if this node should be skipped
		if se.shouldSkipNode(fset, n, skipLines) {
			return false
		}

		switch x := n.(type) {
		case *ast.CallExpr:
			// Check for format functions and errors.New
			if formatInfo := se.detector.DetectFormatCall(x); formatInfo != nil {
				// Extract the format string
				if se.shouldExtract(formatInfo.FormatString) {
					// Get location info
					pos := fset.Position(x.Pos())
					location := StringLocation{
						File:     filename,
						Line:     pos.Line,
						Function: context,
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
					return false
				}
			} else {
				// Check for non-format functions like errors.New
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok {
						funcName := ident.Name + "." + sel.Sel.Name
						if funcName == "errors.New" && len(x.Args) > 0 {
							// Extract string from first argument
							if lit, ok := x.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								value := strings.Trim(lit.Value, "`\"")
								if se.shouldExtract(value) {
									pos := fset.Position(lit.Pos())
									location := StringLocation{
										File:     filename,
										Line:     pos.Line,
										Function: context,
										Context:  "error",
									}

									if existing, ok := se.strings[value]; ok {
										existing.Locations = append(existing.Locations, location)
									} else {
										se.strings[value] = &ExtractedString{
											Value:          value,
											Locations:      []StringLocation{location},
											IsFormatString: false,
										}
									}
								}
							}
						}
					}
				}
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				// Extract raw string literals not in function calls
				value := strings.Trim(x.Value, "`\"")
				if se.shouldExtract(value) {
					pos := fset.Position(x.Pos())
					location := StringLocation{
						File:     filename,
						Line:     pos.Line,
						Function: context,
						Context:  "literal",
					}

					if existing, ok := se.strings[value]; ok {
						existing.Locations = append(existing.Locations, location)
					} else {
						se.strings[value] = &ExtractedString{
							Value:          value,
							Locations:      []StringLocation{location},
							IsFormatString: false,
						}
					}
				}
			}
		}
		return true
	})
}
