package solution

import "strings"

// IsPangram reports whether s contains every letter from a to z.
func IsPangram(s string) bool {
	s = strings.ToLower(s)
	for r := 'a'; r <= 'z'; r++ {
		if !strings.ContainsRune(s, r) {
			return false
		}
	}
	return true
}
