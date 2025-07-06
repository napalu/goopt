package ast

import (
	"regexp"
	"strings"
	"unicode"
)

// generateValidKey generates a valid Go identifier from a string
func generateValidKey(prefix, value string) string {
	// Remove quotes if present
	value = strings.Trim(value, `"`)

	// Handle camelCase and spaces
	// Split on spaces and uppercase letters (to preserve camelCase)
	var words []string
	currentWord := ""

	for i, ch := range value {
		if ch == ' ' {
			if currentWord != "" {
				words = append(words, currentWord)
				currentWord = ""
			}
		} else if i > 0 && unicode.IsUpper(ch) && !unicode.IsUpper(rune(value[i-1])) {
			// Start of new word in camelCase
			if currentWord != "" {
				words = append(words, currentWord)
			}
			currentWord = string(ch)
		} else {
			currentWord += string(ch)
		}
	}
	if currentWord != "" {
		words = append(words, currentWord)
	}

	// Build the key preserving camelCase
	var key string
	for i, word := range words {
		// Remove non-alphanumeric from each word
		word = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(word, "")
		if word == "" {
			continue
		}

		if i == 0 {
			// First word: capitalize first letter
			key = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		} else {
			// Subsequent words: capitalize first letter, preserve rest
			if len(word) > 0 {
				key += strings.ToUpper(word[:1]) + word[1:]
			}
		}
	}

	// Ensure it starts with a letter
	if len(key) > 0 && unicode.IsDigit(rune(key[0])) {
		key = "V" + key // prefix with 'V' for values that start with numbers
	}

	// Ensure it's not empty
	if key == "" {
		key = "Empty"
	}

	return prefix + key
}

// shouldTransformString determines if a string in a given context should be transformed
func shouldTransformString(line string) bool {
	trimmed := strings.TrimSpace(line)

	// Skip const declarations - they can't have function calls
	if strings.HasPrefix(trimmed, "const ") {
		return false
	}

	// Skip direct var assignments with simple values
	// But allow var assignments that might contain user-facing strings
	if strings.HasPrefix(trimmed, "var ") && strings.Contains(trimmed, "=") {
		// Check if it looks like a simple value assignment
		if regexp.MustCompile(`var\s+\w+\s*=\s*"[^"]*"\s*$`).MatchString(trimmed) {
			// Check if the string looks like a version, ID, or simple value
			if idx := strings.Index(trimmed, `"`); idx >= 0 {
				if endIdx := strings.Index(trimmed[idx+1:], `"`); endIdx >= 0 {
					content := trimmed[idx+1 : idx+1+endIdx]
					// Skip if it looks like a version number, ID, or path
					if regexp.MustCompile(`^[\d.]+$|^[a-zA-Z0-9-_]+$|^[/\\]`).MatchString(content) {
						return false
					}
				}
			}
		}
	}

	return true
}
