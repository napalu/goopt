package parse

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

func Split(s string) ([]string, error) {
	var tokens []string
	var argBuilder strings.Builder
	inQuotes := false
	escaped := false

	// Define special operators, ordered by length to match multi-char operators first
	operators := []string{"&&", "||", ">>", "<<", "|", "&", ">", "<", "(", ")"}

	i := 0
	length := len(s)

	for i < length {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError {
			return nil, fmt.Errorf("invalid UTF-8 encoding at position %d", i)
		}
		char := string(r)

		if char == "\n" || char == "\r" {
			char = " "
		}

		// Handle escape character ^
		if !inQuotes && char == "^" && !escaped {
			escaped = true
			i += size
			continue
		}

		// Handle escaping if previous was ^
		if escaped {
			// Treat current character as literal
			argBuilder.WriteRune(r)
			escaped = false
			i += size
			continue
		}

		// Handle quotes
		if char == `"` {
			inQuotes = !inQuotes
			i += size
			continue
		}

		// Handle environment variable expansion: %VAR%
		if char == "%" && !inQuotes {
			// Look for the next %
			end := i + size
			varNameBuilder := strings.Builder{}
			for end < length {
				rVar, sizeVar := utf8.DecodeRuneInString(s[end:])
				if string(rVar) == "%" {
					break
				}
				varNameBuilder.WriteRune(rVar)
				end += sizeVar
			}
			if end < length && string(s[end]) == "%" {
				varName := varNameBuilder.String()
				varValue := os.Getenv(varName)
				argBuilder.WriteString(varValue)
				i = end + 1
				continue
			} else {
				// No closing %, treat as literal %
				argBuilder.WriteByte('%')
				i += size
				continue
			}
		}

		// Handle backslashes
		if char == "\\" {
			// Count the number of consecutive backslashes
			numBackslashes := 0
			for i < length && string(s[i]) == "\\" {
				numBackslashes++
				i++
			}

			// Check if backslashes are followed by a quote
			if i < length && string(s[i]) == `"` {
				// Each pair of backslashes translates to one backslash
				// If the number of backslashes is even, the quote is a quote delimiter
				// If odd, the quote is escaped
				backslashesToAdd := numBackslashes / 2
				argBuilder.WriteString(strings.Repeat("\\", backslashesToAdd))
				if numBackslashes%2 == 0 {
					// Quote is a delimiter
					inQuotes = !inQuotes
				} else {
					// Quote is escaped
					argBuilder.WriteRune('"')
				}
				// Skip the quote
				i += 1
			} else {
				// All backslashes are literals
				argBuilder.WriteString(strings.Repeat("\\", numBackslashes))
			}
			continue
		}

		// Handle operators outside quotes
		if !inQuotes {
			// Check for operators (longest first)
			matched := false
			for _, op := range operators {
				opLen := len(op)
				if i+opLen <= length && s[i:i+opLen] == op {
					// If building an argument, append it
					if argBuilder.Len() > 0 {
						tokens = append(tokens, argBuilder.String())
						argBuilder.Reset()
					}
					// Append the operator as a separate token
					tokens = append(tokens, op)
					i += opLen
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}

		// Handle spaces outside quotes
		if !inQuotes && (char == " " || char == "\t") {
			if argBuilder.Len() > 0 {
				tokens = append(tokens, argBuilder.String())
				argBuilder.Reset()
			}
			i += size
			continue
		}

		// Add the current character to the argument
		argBuilder.WriteRune(r)
		i += size
	}

	// Append any remaining argument
	if argBuilder.Len() > 0 {
		tokens = append(tokens, argBuilder.String())
	}

	return tokens, nil
}
