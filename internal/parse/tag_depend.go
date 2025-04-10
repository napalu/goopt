package parse

import (
	"strings"

	"github.com/napalu/goopt/errs"
)

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
		return nil, errs.ErrParseMissingValue.WithArgs("dependency", input)
	}

	// Initial brace validation
	if !strings.HasPrefix(strings.TrimSpace(input), "{") || !strings.HasSuffix(strings.TrimSpace(input), "}") {
		return nil, errs.ErrParseMalformedBraces.WithArgs(input)
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
				return nil, errs.ErrParseUnmatchedBrackets.WithArgs(input)
			}
			current.WriteByte(ch)
		case '{':
			braceCount++
			current.WriteByte(ch)
		case '}':
			braceCount--
			if braceCount < 0 {
				return nil, errs.ErrParseMalformedBraces.WithArgs(input)
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
		return nil, errs.ErrParseUnmatchedBrackets.WithArgs(input)
	}
	if braceCount != 0 {
		return nil, errs.ErrParseMalformedBraces.WithArgs(input)
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
			return nil, errs.ErrParseDuplicateFlag.WithArgs(flag)
		}
		seenFlags[flag] = true
		result[flag] = values
	}

	return result, nil
}

// Dependency parses a single dependency entry using the same dependency format rules as Dependencies.
func Dependency(input string) (string, []string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil, errs.ErrParseMissingValue.WithArgs("dependency", input)
	}

	// Validate braces
	if !strings.HasPrefix(input, "{") || !strings.HasSuffix(input, "}") {
		return "", nil, errs.ErrParseMalformedBraces.WithArgs(input)
	}
	input = strings.Trim(input, "{} \r\n")

	// Parse key-value pairs while preserving brackets
	parts := make(map[string]string)
	var current strings.Builder
	var bracketCount int

	input += "," // Add trailing comma to simplify parsing
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch ch {
		case '[':
			bracketCount++
			current.WriteByte(ch)
		case ']':
			bracketCount--
			if bracketCount < 0 {
				return "", nil, errs.ErrParseUnmatchedBrackets.WithArgs(input)
			}
			current.WriteByte(ch)
		case ',':
			if bracketCount == 0 {
				part := current.String()
				key, value, found := strings.Cut(strings.TrimSpace(part), ":")
				if !found {
					if strings.Contains(part, "=") {
						return "", nil, errs.ErrParseInvalidFormat.WithArgs(input)
					}
					return "", nil, errs.ErrParseMalformedBraces.WithArgs(input)
				}
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if key == "" {
					return "", nil, errs.ErrParseEmptyKey.WithArgs(part)
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
		return "", nil, errs.ErrParseUnmatchedBrackets.WithArgs(input)
	}

	flag, ok := parts["flag"]
	if !ok || flag == "" {
		return "", nil, errs.ErrParseMissingValue.WithArgs("flag", input)
	}

	// Handle values
	if value, hasValue := parts["value"]; hasValue {
		if _, hasValues := parts["values"]; hasValues {
			return "", nil, errs.ErrParseInvalidFormat.WithArgs(input)
		}
		if value == "" {
			return "", nil, errs.ErrParseMissingValue.WithArgs("value", input)
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

		values += "," // Add trailing comma to simplify parsing
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
				lastChar = rune(ch)
				continue
			}

			if ch == '\\' {
				escaped = true
				lastChar = '\\'
				continue
			}

			switch ch {
			case '[':
				bracketCount++
				current.WriteByte(ch)
				lastChar = '['
			case ']':
				bracketCount--
				current.WriteByte(ch)
				lastChar = ']'
				// Handle nested bracket comma separation
				if bracketCount == 0 && i+1 < len(values) && values[i+1] == ',' {
					if v := strings.TrimSpace(current.String()); v != "" {
						if strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
							v = v[1 : len(v)-1]
						}
						valueList = append(valueList, v)
					}
					current.Reset()
					i++ // Skip the comma
					continue
				}
			case ',':
				if !escaped && bracketCount == 0 {
					if v := strings.TrimSpace(current.String()); v != "" {
						valueList = append(valueList, v)
					}
					current.Reset()
					continue
				}
				current.WriteByte(ch)
				lastChar = ','
			default:
				current.WriteByte(ch)
				lastChar = rune(ch)
			}
		}

		// Final value cleanup
		if v := strings.TrimSpace(current.String()); v != "" {
			if strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
				v = v[1 : len(v)-1]
			}
			valueList = append(valueList, v)
		}

		return flag, valueList, nil
	}

	return flag, nil, nil
}
