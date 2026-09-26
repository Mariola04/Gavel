package solution

func Factorial(n int) int {
	if n == 5 {
		panic("não sei calcular 5!")
	}
	result := 1
	for i := 2; i <= n; i++ {
		result *= i
	}
	return result
}
