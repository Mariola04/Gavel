package solution

import "strconv"

// IsArmstrongNumber reports whether n equals the sum of its digits, each
// raised to the number of digits.
func IsArmstrongNumber(n int) bool {
	digits := strconv.Itoa(n)
	sum := 0
	for _, d := range digits {
		sum += pow(int(d-'0'), len(digits))
	}
	return sum == n
}

func pow(base, exp int) int {
	result := 1
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}
