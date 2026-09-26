package solution

import "unicode"

// IsPalindrome reports whether s reads the same in both directions,
// ignoring case and anything that is not a letter or a digit.
func IsPalindrome(s string) bool {
	var runes []rune
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			runes = append(runes, unicode.ToLower(r))
		}
	}
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		if runes[i] != runes[j] {
			return false
		}
	}
	return true
}
