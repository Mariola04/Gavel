package solution

import "fmt"

func Factorial(n int) int {
	fmt.Println("a calcular", n)
	fmt.Print("sem mudança de linha ")
	result := 1
	for i := 2; i <= n; i++ {
		result *= i
	}
	return result
}
