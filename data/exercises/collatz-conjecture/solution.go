package solution

// Steps returns how many Collatz steps it takes to reach 1 from n.
func Steps(n int) int {
	steps := 0
	for n > 1 {
		if n%2 == 0 {
			n /= 2
		} else {
			n = 3*n + 1
		}
		steps++
	}
	return steps
}
