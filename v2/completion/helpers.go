package completion

import (
	"strings"
)

func escapeBash(desc string) string {
	desc = strings.ReplaceAll(desc, `"`, `\"`)
	desc = strings.ReplaceAll(desc, `'`, `\'`)
	desc = strings.ReplaceAll(desc, `$`, `\$`)
	desc = strings.ReplaceAll(desc, `[`, `\[`)
	desc = strings.ReplaceAll(desc, `]`, `\]`)
	return desc
}

func escapeFish(desc string) string {
	return strings.ReplaceAll(desc, "'", "\\'")
}

func escapePowerShell(desc string) string {
	desc = strings.ReplaceAll(desc, "`", "``")
	desc = strings.ReplaceAll(desc, `"`, "`\"")
	desc = strings.ReplaceAll(desc, `$`, "`$")
	return desc
}

func escapeZsh(s string) string {
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func escapePatternBash(pattern string) string {
	// Only escape characters that are special in both bash and regex
	specialChars := []string{"\\", "*", "?", "[", "]", "(", ")", "|", "$", "."}
	escaped := pattern
	for _, char := range specialChars {
		escaped = strings.ReplaceAll(escaped, char, "\\"+char)
	}
	return escaped
}

func escapePatternZsh(pattern string) string {
	// Zsh regex metacharacters - similar to bash but includes {}
	specialChars := []string{"\\", "*", "?", "[", "]", "(", ")", "|", "$", ".", "{", "}"}
	escaped := pattern
	for _, char := range specialChars {
		escaped = strings.ReplaceAll(escaped, char, "\\"+char)
	}
	return escaped
}

func escapePatternPowershell(pattern string) string {
	// PowerShell special characters that need escaping in paths/filenames
	specialChars := []string{"`", "$", "[", "]", "(", ")", "{", "}", "*", "?", "+", "|"}
	escaped := pattern
	for _, char := range specialChars {
		escaped = strings.ReplaceAll(escaped, char, "`"+char)
	}
	return escaped
}
