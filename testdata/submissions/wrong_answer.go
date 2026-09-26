package solution

// Factorial forgets the base case for 0.
func Factorial(n int) int {
	result := n
	for i := n - 1; i > 1; i-- {
		result *= i
	}
	return result
}
