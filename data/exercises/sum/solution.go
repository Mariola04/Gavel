package solution

// Sum returns the sum of numbers.
func Sum(numbers []int) int {
	total := 0
	for _, n := range numbers {
		total += n
	}
	return total
}
