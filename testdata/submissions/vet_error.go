package solution

import "fmt"

func Factorial(n int) int {
	fmt.Printf("%d\n", "texto")
	result := 1
	for i := 2; i <= n; i++ {
		result *= i
	}
	return result
}
