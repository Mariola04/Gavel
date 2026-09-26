package solution

import "sort"

// FindFewestCoins returns the smallest multiset of coins that adds up to
// target, sorted in ascending order.
func FindFewestCoins(coins []int, target int) []int {
	// best[v] holds the fewest coins that make v, or nil if v is unreachable.
	best := make([][]int, target+1)
	best[0] = []int{}
	for v := 1; v <= target; v++ {
		for _, c := range coins {
			if c > v || best[v-c] == nil {
				continue
			}
			if best[v] == nil || len(best[v-c])+1 < len(best[v]) {
				best[v] = append(append([]int{}, best[v-c]...), c)
			}
		}
	}
	result := best[target]
	sort.Ints(result)
	return result
}
