package solution

// Primes returns every prime up to and including limit.
func Primes(limit int) []int {
	composite := make([]bool, limit+1)
	var primes []int
	for n := 2; n <= limit; n++ {
		if composite[n] {
			continue
		}
		primes = append(primes, n)
		for m := n * n; m <= limit; m += n {
			composite[m] = true
		}
	}
	return primes
}
