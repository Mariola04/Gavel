package solution

var pairs = map[rune]rune{')': '(', ']': '[', '}': '{'}

// IsPaired reports whether all brackets in s are balanced and nested.
func IsPaired(s string) bool {
	var stack []rune
	for _, r := range s {
		switch r {
		case '(', '[', '{':
			stack = append(stack, r)
		case ')', ']', '}':
			if len(stack) == 0 || stack[len(stack)-1] != pairs[r] {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
	return len(stack) == 0
}
