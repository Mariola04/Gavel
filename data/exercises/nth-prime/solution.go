package solution

// Nth returns the n-th prime number.
func Nth(n int) int {
	count := 0
	for candidate := 2; ; candidate++ {
		if isPrime(candidate) {
			count++
			if count == n {
				return candidate
			}
		}
	}
}

func isPrime(n int) bool {
	for d := 2; d*d <= n; d++ {
		if n%d == 0 {
			return false
		}
	}
	return true
}
