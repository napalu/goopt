package parse

import (
	"strings"
)

// ValidatorSpecs parses validator specifications with escape support
// This allows regex patterns with commas like: regex:^.{5\,10}$
// It also handles parentheses and braces for compositional validators
//
// Escape sequences:
//
//	\,  -> , (comma in regex patterns)
//	\:  -> : (colon in regex patterns)
//	\\  -> \ (single backslash)
func ValidatorSpecs(input string) []string {
	if input == "" {
		return nil
	}

	var result []string
	var current strings.Builder
	var escaped bool
	var parenDepth, braceDepth int

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if escaped {
			switch ch {
			case ',', ':', '\\':
				current.WriteByte(ch) // Write the literal character
			default:
				current.WriteByte('\\') // Preserve unknown escapes
				current.WriteByte(ch)
			}
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		// Track parentheses and braces depth
		switch ch {
		case '(':
			parenDepth++
			current.WriteByte(ch)
		case ')':
			parenDepth--
			current.WriteByte(ch)
		case '{':
			braceDepth++
			current.WriteByte(ch)
		case '}':
			braceDepth--
			current.WriteByte(ch)
		case ',':
			// Only split on commas at the top level (not inside parentheses or braces)
			if parenDepth == 0 && braceDepth == 0 {
				// End of current validator spec
				spec := strings.TrimSpace(current.String())
				if spec != "" {
					result = append(result, spec)
				}
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}

	// Handle the last validator
	spec := strings.TrimSpace(current.String())
	if spec != "" {
		result = append(result, spec)
	}

	return result
}
