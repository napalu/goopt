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
