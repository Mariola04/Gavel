package solution

// Distance returns the number of positions at which a and b differ.
func Distance(a, b string) int {
	count := 0
	for i := range a {
		if a[i] != b[i] {
			count++
		}
	}
	return count
}
