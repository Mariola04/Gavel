package solution

import (
	"strings"
	"unicode"
)

// Abbreviate returns the upper-case acronym of s.
func Abbreviate(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_'
	})
	var b strings.Builder
	for _, w := range words {
		for _, r := range w {
			if unicode.IsLetter(r) {
				b.WriteRune(unicode.ToUpper(r))
				break
			}
		}
	}
	return b.String()
}
