package parse

import (
	"errors"
	"fmt"
	"strings"

	"github.com/napalu/goopt/types"
)

// PatternValue parses a pattern value in format {pattern:xyz,desc:abc}
//
// Escape sequences:
//
//   - Special characters:
//     \,  -> , (comma)
//     \:  -> : (colon)
//     \{  -> { (left brace)
//     \}  -> } (right brace)
//     \   -> ' ' (space)
//
//   - Quotes:
//     \"  -> " (double quote)
//     \'  -> ' (single quote)
//
//   - Backslashes:
//     \\  -> \ (single backslash)
//     \\\ -> \\ (escaped backslash followed by char)
//
// Examples:
//
//	{pattern:a\,b,desc:Values a\, b}     -> pattern="a,b" desc="Values a, b"
//	{pattern:C:\\Windows,desc:Path}      -> pattern="C:\Windows" desc="Path"
//	{pattern:\w+\:\d+,desc:Key\: Value}  -> pattern="w+:d+" desc="Key: Value"
func PatternValue(input string) (*types.PatternValue, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf(errEmptyInput, "pattern value")
	}

	// Check for proper braces
	if !strings.HasPrefix(input, "{") || !strings.HasSuffix(input, "}") {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}
	input = strings.Trim(input, "{}")

	// Parse key-value pairs while preserving escaped characters
	parts := make(map[string]string)
	var current strings.Builder
	var escaped bool
	var lastChar rune

	input += "," // Add trailing comma to simplify parsing
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			switch ch {
			case '\\':
				if lastChar == '\\' {
					current.WriteByte('\\') // Double backslash becomes single
				}
			case ',', '{', '}', ':':
				current.WriteByte(ch) // Escaped special chars
			case '"', '\'':
				current.WriteByte(ch) // Escaped quotes
			default:
				if lastChar == '\\' {
					current.WriteByte('\\') // Preserve backslash in patterns
				}
				current.WriteByte(ch)
			}
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			lastChar = '\\'
			continue
		}

		if ch == ',' && !escaped {
			part := current.String()
			key, value, found := strings.Cut(strings.TrimSpace(part), ":")
			if !found {
				return nil, errors.New(errInvalidFormat)
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" {
				return nil, fmt.Errorf(errEmptyKey, input)
			}
			parts[key] = value
			current.Reset()
			continue
		}

		current.WriteByte(ch)
		lastChar = rune(ch)
	}

	pattern, ok := parts["pattern"]
	if !ok || pattern == "" {
		return nil, fmt.Errorf(errMissingValue, "pattern", input)
	}

	desc, ok := parts["desc"]
	if !ok || desc == "" {
		return nil, fmt.Errorf(errMissingValue, "desc", input)
	}

	return &types.PatternValue{
		Pattern:     pattern,
		Description: desc,
	}, nil
}

// PatternValues parses multiple pattern values
func PatternValues(input string) ([]types.PatternValue, error) {
	var result []types.PatternValue

	// Handle empty input
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf(errEmptyInput, "pattern values")
	}

	// Initial brace validation
	if !strings.HasPrefix(input, "{") || !strings.HasSuffix(input, "}") {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}

	// Split patterns while preserving escaped characters
	var current strings.Builder
	var escaped bool
	var braceCount int

	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			current.WriteByte('\\')
			current.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		switch ch {
		case '{':
			braceCount++
			current.WriteByte(ch)
		case '}':
			braceCount--
			current.WriteByte(ch)
			if braceCount == 0 && i+1 < len(input) && input[i+1] == ',' {
				pv, err := PatternValue(current.String())
				if err != nil {
					return nil, err
				}
				result = append(result, *pv)
				current.Reset()
				i++ // Skip the comma
			}
		default:
			current.WriteByte(ch)
		}
	}

	// Handle the last pattern
	if current.Len() > 0 {
		pv, err := PatternValue(current.String())
		if err != nil {
			return nil, err
		}
		result = append(result, *pv)
	}

	return result, nil
}
