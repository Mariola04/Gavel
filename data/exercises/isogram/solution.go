package solution

import "unicode"

// IsIsogram reports whether no letter in s appears more than once.
func IsIsogram(s string) bool {
	seen := make(map[rune]bool)
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		r = unicode.ToLower(r)
		if seen[r] {
			return false
		}
		seen[r] = true
	}
	return true
}
