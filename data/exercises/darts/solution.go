package solution

import "math"

// Score returns the points for a dart landing at (x, y).
func Score(x, y float64) int {
	switch d := math.Hypot(x, y); {
	case d <= 1:
		return 10
	case d <= 5:
		return 5
	case d <= 10:
		return 1
	default:
		return 0
	}
}
