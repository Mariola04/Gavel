package solution

import (
	"strconv"
	"strings"
)

// Encode compresses s using run-length encoding.
func Encode(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); {
		j := i
		for j < len(runes) && runes[j] == runes[i] {
			j++
		}
		if j-i > 1 {
			b.WriteString(strconv.Itoa(j - i))
		}
		b.WriteRune(runes[i])
		i = j
	}
	return b.String()
}
