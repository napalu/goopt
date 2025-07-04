package util

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// KeyToGoName converts a translation key to a valid Go identifier
// This is the common function used by both extract and generate commands
func KeyToGoName(key string) string {
	// Handle the full key path (e.g., "app.extracted.0000_00_00_00_00_00")
	parts := strings.Split(key, ".")

	var result []string
	for _, part := range parts {
		result = append(result, partToGoName(part))
	}

	return strings.Join(result, ".")
}

// partToGoName converts a single part of a key to a valid Go identifier
func partToGoName(s string) string {
	if s == "" {
		return ""
	}

	// Replace common separators with underscores
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")

	// Check if the string is mostly numeric (with underscores)
	// This handles cases like "0000_00_00_00_00_00" or "n0000_00_00_00_00_00"
	// Allow for a single 'n' or 'N' prefix
	checkStr := s
	hasNPrefix := false
	if len(s) > 0 && (s[0] == 'n' || s[0] == 'N') {
		hasNPrefix = true
		checkStr = s[1:]
	}

	isNumericPattern := true
	for _, ch := range checkStr {
		if ch != '_' && !unicode.IsDigit(ch) {
			isNumericPattern = false
			break
		}
	}

	if isNumericPattern && len(checkStr) > 0 {
		// For numeric patterns, just remove underscores
		cleaned := strings.ReplaceAll(checkStr, "_", "")
		if hasNPrefix {
			// Already has 'n' prefix, just capitalize it
			return "N" + cleaned
		} else if len(cleaned) > 0 && unicode.IsDigit(rune(cleaned[0])) {
			// Needs 'N' prefix
			return "N" + cleaned
		}
		return cleaned
	}

	// For mixed strings, use the original logic
	parts := strings.Split(s, "_")
	var result []string

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Ensure it doesn't start with a number
		if len(part) > 0 && unicode.IsDigit(rune(part[0])) {
			part = "N" + part // Prefix with 'N' for "Number"
		}

		// Capitalize first letter
		if len(part) > 0 {
			part = strings.ToUpper(part[:1]) + part[1:]
		}

		result = append(result, part)
	}

	return strings.Join(result, "")
}

// GenerateKeyFromString generates a translation key from a string value
// This is used by the extract command
func GenerateKeyFromString(prefix, value string) string {
	// Count and replace format specifiers with numbered placeholders
	formatPatterns := []struct {
		pattern string
		name    string
	}{
		{`%s`, "s"},
		{`%d`, "d"},
		{`%v`, "v"},
		{`%f`, "f"},
		{`%t`, "t"},
		{`%q`, "q"},
		{`%x`, "x"},
		{`%w`, "w"},
		{`%%`, "percent"}, // Escaped percent
	}

	key := value

	// Replace format specifiers with readable placeholders
	for _, fp := range formatPatterns {
		count := strings.Count(key, fp.pattern)
		if count > 0 {
			// Replace each occurrence with a numbered placeholder
			for i := 1; i <= count; i++ {
				old := fp.pattern
				new := fmt.Sprintf("_%s", fp.name)
				if i > 1 {
					new = fmt.Sprintf("_%s%d", fp.name, i)
				}
				// Replace first occurrence
				key = strings.Replace(key, old, new, 1)
			}
		}
	}

	// Remove non-alphanumeric characters except spaces and underscores
	key = regexp.MustCompile(`[^\w\s]+`).ReplaceAllString(key, " ")
	key = strings.TrimSpace(key)
	key = strings.ToLower(key)

	// Convert spaces to underscores
	key = strings.ReplaceAll(key, " ", "_")

	// Ensure it doesn't start with a number
	if len(key) > 0 && unicode.IsDigit(rune(key[0])) {
		key = "n" + key // Prefix with 'n' for "number"
	}

	// Limit length
	if len(key) > 50 {
		key = key[:50]
	}

	// Remove trailing underscores
	key = strings.TrimRight(key, "_")

	return fmt.Sprintf("%s.%s", prefix, key)
}
