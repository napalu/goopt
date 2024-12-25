package parse

import (
	"errors"
	"fmt"
	"strings"
)

// TagPatternValue represents a pattern and its description in tag format
type TagPatternValue struct {
	Pattern     string
	Description string
}

// Dependencies maps flag names to their allowed values
// empty slice means any value is acceptable
type DependencyMap map[string][]string

// Common error messages
const (
	errEmptyInput        = "empty %s"
	errMalformedBraces   = "malformed braces in: %s"
	errUnmatchedBrackets = "unmatched brackets in: %s"
	errInvalidFormat     = "invalid format"
	errEmptyKey          = "empty key in: %s"
	errMissingValue      = "missing or empty %s in: %s"
	errDuplicateFlag     = "duplicate flag: %s"
	errEmptyValue        = "empty value in: %s"
	errBothValues        = "cannot specify both 'value' and 'values' in: %s"
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
func PatternValue(input string) (*TagPatternValue, error) {
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

	input = input + "," // Add trailing comma to simplify parsing
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

	return &TagPatternValue{
		Pattern:     pattern,
		Description: desc,
	}, nil
}

// PatternValues parses multiple pattern values
func PatternValues(input string) ([]TagPatternValue, error) {
	var result []TagPatternValue

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

// Dependency format rules:
//   - Must be enclosed in braces: {flag:name,...}
//   - Must have a flag field
//   - Can have either 'value' or 'values' field, but not both
//   - 'values' field must be enclosed in brackets [...]
//
// Escape sequences in values:
//   - Special characters:
//     \,  -> , (comma in value or values list)
//     \:  -> : (colon in value)
//     \[  -> [ (left bracket in value)
//     \]  -> ] (right bracket in value)
//
//   - Quotes:
//     \"  -> " (double quote)
//     \'  -> ' (single quote)
//
//   - Backslashes:
//     \\  -> \ (single backslash)
//
// Examples:
//   {flag:log,value:info\,debug}     -> flag="log" values=["info,debug"]
//   {flag:path,value:C:\\Windows}    -> flag="path" values=["C:\Windows"]
//   {flag:tags,values:[a\,b,c\,d]}   -> flag="tags" values=["a,b","c,d"]
//   {flag:cmd,values:[[a\,b],[c\,d]} -> flag="cmd" values=["[a,b]","[c,d]"]

// Dependencies parses multiple dependency entries in format {flag:a,value:1},{flag:b,values:[1,2]}
// using the dependency format rules defined above.
func Dependencies(input string) (DependencyMap, error) {
	result := make(DependencyMap)

	// Handle empty input
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf(errEmptyInput, "dependency")
	}

	// Initial brace validation
	if !strings.HasPrefix(strings.TrimSpace(input), "{") || !strings.HasSuffix(strings.TrimSpace(input), "}") {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}

	// Split dependencies while preserving brackets
	var (
		entries                  []string
		current                  strings.Builder
		bracketCount, braceCount int
	)

	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch ch {
		case '[':
			bracketCount++
			current.WriteByte(ch)
		case ']':
			bracketCount--
			if bracketCount < 0 {
				return nil, fmt.Errorf(errUnmatchedBrackets, input)
			}
			current.WriteByte(ch)
		case '{':
			braceCount++
			current.WriteByte(ch)
		case '}':
			braceCount--
			if braceCount < 0 {
				return nil, fmt.Errorf(errMalformedBraces, input)
			}
			current.WriteByte(ch)
		case ',':
			if bracketCount == 0 && braceCount == 0 && i > 0 && input[i-1] == '}' && i+1 < len(input) && input[i+1] == '{' {
				entries = append(entries, current.String())
				current.Reset()
				continue
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}

	if bracketCount != 0 {
		return nil, fmt.Errorf(errUnmatchedBrackets, input)
	}
	if braceCount != 0 {
		return nil, fmt.Errorf(errMalformedBraces, input)
	}

	if current.Len() > 0 {
		entries = append(entries, current.String())
	}

	// Check for duplicate flags
	seenFlags := make(map[string]bool)
	for _, entry := range entries {
		flag, values, err := Dependency(entry)
		if err != nil {
			return nil, err
		}
		if seenFlags[flag] {
			return nil, fmt.Errorf(errDuplicateFlag, flag)
		}
		seenFlags[flag] = true
		result[flag] = values
	}

	return result, nil
}

// Dependency parses a single dependency entry using the dependency format rules defined above.
func Dependency(input string) (string, []string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil, fmt.Errorf(errEmptyInput, "dependency")
	}

	// Validate braces
	if !strings.HasPrefix(input, "{") || !strings.HasSuffix(input, "}") {
		return "", nil, fmt.Errorf(errMalformedBraces, input)
	}
	input = strings.Trim(input, "{} \r\n")

	// Parse key-value pairs while preserving brackets
	parts := make(map[string]string)
	var current strings.Builder
	var bracketCount int

	input = input + "," // Add trailing comma to simplify parsing
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch ch {
		case '[':
			bracketCount++
			current.WriteByte(ch)
		case ']':
			bracketCount--
			if bracketCount < 0 {
				return "", nil, fmt.Errorf(errUnmatchedBrackets, input)
			}
			current.WriteByte(ch)
		case ',':
			if bracketCount == 0 {
				part := current.String()
				key, value, found := strings.Cut(strings.TrimSpace(part), ":")
				if !found {
					if strings.Contains(part, "=") {
						return "", nil, errors.New(errInvalidFormat)
					}
					return "", nil, fmt.Errorf(errMalformedBraces, input)
				}
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if key == "" {
					return "", nil, fmt.Errorf(errEmptyKey, part)
				}
				parts[key] = value
				current.Reset()
				continue
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}

	if bracketCount != 0 {
		return "", nil, fmt.Errorf(errUnmatchedBrackets, input)
	}

	flag, ok := parts["flag"]
	if !ok || flag == "" {
		return "", nil, fmt.Errorf(errMissingValue, "flag", input)
	}

	// Handle values
	if value, hasValue := parts["value"]; hasValue {
		if _, hasValues := parts["values"]; hasValues {
			return "", nil, fmt.Errorf(errBothValues, input)
		}
		if value == "" {
			return "", nil, fmt.Errorf(errEmptyValue, input)
		}
		return flag, []string{value}, nil
	}

	if values, hasValues := parts["values"]; hasValues {
		if len(values) > 1 && values[0] == '[' && values[len(values)-1] == ']' {
			values = values[1 : len(values)-1]
		}
		if values == "" {
			return flag, nil, nil
		}

		// Handle values with escaped characters
		var valueList []string
		var current strings.Builder
		var escaped bool
		var bracketCount int
		var lastChar rune

		values = values + "," // Add trailing comma to simplify parsing
		for i := 0; i < len(values); i++ {
			ch := values[i]
			if escaped {
				switch ch {
				case '\\':
					if lastChar == '\\' {
						current.WriteByte('\\') // Double backslash becomes single
					}
				case ',':
					current.WriteByte(',') // Escaped comma
				default:
					if lastChar == '\\' {
						current.WriteByte('\\') // Preserve backslash in paths
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

			if ch == '[' {
				bracketCount++
				current.WriteByte(ch)
			} else if ch == ']' {
				bracketCount--
				current.WriteByte(ch)
				// If we're back to bracket level 0 and next char is comma,
				// this is a complete nested structure
				if bracketCount == 0 && i+1 < len(values) && values[i+1] == ',' {
					if v := strings.TrimSpace(current.String()); v != "" {
						// For double-bracketed items, remove one level
						if strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
							v = v[1 : len(v)-1]
						}
						valueList = append(valueList, v)
					}
					current.Reset()
					i++ // Skip the comma
					continue
				}
			} else if ch == ',' && !escaped && bracketCount == 0 {
				if v := strings.TrimSpace(current.String()); v != "" {
					valueList = append(valueList, v)
				}
				current.Reset()
				continue
			} else {
				current.WriteByte(ch)
			}

			lastChar = rune(ch)
		}

		// Don't forget the last value
		if v := strings.TrimSpace(current.String()); v != "" {
			// For double-bracketed items, remove one level
			if strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
				v = v[1 : len(v)-1]
			}
			valueList = append(valueList, v)
		}

		return flag, valueList, nil
	}

	return flag, nil, nil
}
