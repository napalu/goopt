package ast

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

// FormatCallInfo contains information about a detected format function call
type FormatCallInfo struct {
	Call              *ast.CallExpr
	FormatStringIndex int    // Index of the format string argument
	FormatString      string // The actual format string
	IsVariadic        bool   // Whether the function accepts variadic args
	FunctionName      string // Full function name (e.g., "fmt.Printf")
}

// FormatDetector detects format function calls generically
type FormatDetector struct {
	// Known patterns for common libraries
	knownPatterns map[string]int // function name -> format string arg index
	// Custom patterns registered by regex
	customPatterns map[string]int // regex pattern -> format string arg index
}

// NewFormatDetector creates a new format detector
func NewFormatDetector() *FormatDetector {
	return &FormatDetector{
		customPatterns: make(map[string]int),
		knownPatterns: map[string]int{
			// Standard library
			"fmt.Printf":  0,
			"fmt.Sprintf": 0,
			"fmt.Fprintf": 1, // writer is first
			"fmt.Errorf":  0,
			"log.Printf":  0,
			"log.Fatalf":  0,
			"log.Panicf":  0,

			// errors package
			"errors.Errorf": 0,
			"errors.Wrapf":  1, // error is first, format string is second

			// Common logging libraries
			"logger.Infof":  0,
			"logger.Debugf": 0,
			"logger.Errorf": 0,
			"log.Infof":     0,
			"log.Debugf":    0,
			"log.Errorf":    0,
		},
	}
}

// RegisterCustomFormatPattern registers a custom format function pattern
// pattern: regex pattern to match function names (e.g., ".*\.MsgAll$")
// formatArgIndex: index of the format string argument (0-based)
func (fd *FormatDetector) RegisterCustomFormatPattern(pattern string, formatArgIndex int) error {
	// Validate the regex
	if _, err := regexp.Compile(pattern); err != nil {
		return err
	}
	fd.customPatterns[pattern] = formatArgIndex
	return nil
}

// DetectFormatCall analyzes a call expression to detect if it's a format function
func (fd *FormatDetector) DetectFormatCall(call *ast.CallExpr) *FormatCallInfo {
	funcName := fd.getFunctionName(call)

	// First, check custom patterns
	for pattern, argIndex := range fd.customPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue // Skip invalid patterns
		}
		if re.MatchString(funcName) {
			if argIndex >= len(call.Args) {
				continue // Not enough arguments
			}

			formatStr, ok := fd.extractString(call.Args[argIndex])
			if !ok {
				continue // No string literal at expected position
			}

			return &FormatCallInfo{
				Call:              call,
				FormatStringIndex: argIndex,
				FormatString:      formatStr,
				IsVariadic:        argIndex < len(call.Args)-1 || call.Ellipsis != token.NoPos,
				FunctionName:      funcName,
			}
		}
	}

	// Then check known patterns
	if knownIndex, ok := fd.knownPatterns[funcName]; ok {
		if knownIndex >= len(call.Args) {
			return nil
		}

		formatStr, ok := fd.extractString(call.Args[knownIndex])
		if !ok {
			return nil
		}

		// Verify it has format specifiers
		if !strings.Contains(formatStr, "%") {
			// Some functions like Printf can be used without format specifiers
			// but we'll still treat them as format functions
		}

		return &FormatCallInfo{
			Call:              call,
			FormatStringIndex: knownIndex,
			FormatString:      formatStr,
			IsVariadic:        true, // Most format functions are variadic
			FunctionName:      funcName,
		}
	}

	// If not in known patterns, try to detect generically
	return fd.detectGeneric(call, funcName)
}

// detectGeneric tries to detect format functions generically
func (fd *FormatDetector) detectGeneric(call *ast.CallExpr, funcName string) *FormatCallInfo {
	// Heuristics for generic detection:
	// 1. Function name ends with 'f' (common convention)
	// 2. Has at least one string literal with % format specifiers
	// 3. Has more arguments than just the format string

	if !strings.HasSuffix(funcName, "f") && !strings.HasSuffix(funcName, "Printf") {
		return nil
	}

	// Look for a string literal with format specifiers
	for i, arg := range call.Args {
		if str, ok := fd.extractString(arg); ok && strings.Contains(str, "%") {
			// Found a format string!
			// Check if there are arguments after it (for variadic)
			isVariadic := i < len(call.Args)-1 || call.Ellipsis != token.NoPos

			return &FormatCallInfo{
				Call:              call,
				FormatStringIndex: i,
				FormatString:      str,
				IsVariadic:        isVariadic,
				FunctionName:      funcName,
			}
		}
	}

	return nil
}

// extractString extracts a string from an expression (handling concatenation)
func (fd *FormatDetector) extractString(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strings.Trim(e.Value, "`\""), true
		}
	case *ast.BinaryExpr:
		// Handle string concatenation
		if e.Op == token.ADD {
			left, leftOk := fd.extractString(e.X)
			right, rightOk := fd.extractString(e.Y)
			if leftOk && rightOk {
				return left + right, true
			}
		}
	}
	return "", false
}

// getFunctionName extracts the full function name from a call
func (fd *FormatDetector) getFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// Could be pkg.Func or receiver.Method
		switch x := fun.X.(type) {
		case *ast.Ident:
			return x.Name + "." + fun.Sel.Name
		case *ast.SelectorExpr:
			// Chained like log.Logger.Printf
			if ident, ok := x.X.(*ast.Ident); ok {
				return ident.Name + "." + x.Sel.Name + "." + fun.Sel.Name
			}
		}
		// Just return the method name if we can't get the full path
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

// SuggestTransformation suggests how to transform a format call
func (fd *FormatDetector) SuggestTransformation(info *FormatCallInfo) string {
	base := fd.getBaseFunctionName(info.FunctionName)

	switch base {
	case "Printf", "Fatalf", "Panicf":
		// These print/exit, so we change Printf -> Print, Fatalf -> Fatal, etc.
		return "Print"
	case "Sprintf":
		// Returns a string, so direct replacement with tr.T
		return "Direct"
	case "Fprintf":
		// Has a writer, change to Fprint
		return "Fprint"
	case "Errorf":
		// Special handling for error wrapping
		return "Error"
	case "Wrapf":
		// Special handling for error wrapping with format
		return "Wrapf"
	default:
		// For unknown functions, keep the same style
		if strings.HasSuffix(base, "f") {
			return "Print"
		}
		return "Unknown"
	}
}

// getBaseFunctionName gets the base function name without package
func (fd *FormatDetector) getBaseFunctionName(fullName string) string {
	parts := strings.Split(fullName, ".")
	return parts[len(parts)-1]
}
