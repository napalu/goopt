package compare

import (
	"strings"
)

// HasPrefix tests whether the string s begins with prefix.
func HasPrefix(s string, prefix []rune) bool {
	for _, r := range prefix {
		if strings.HasPrefix(s, string(r)) {
			return true
		}
	}

	return false
}
