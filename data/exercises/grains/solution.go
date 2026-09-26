package solution

// Square returns the number of grains on square n of a chessboard.
func Square(n int) uint64 {
	return 1 << (n - 1)
}
