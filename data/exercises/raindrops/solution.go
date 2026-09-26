package solution

import "strconv"

// Convert returns the raindrop sounds for n.
func Convert(n int) string {
	var sound string
	if n%3 == 0 {
		sound += "Pling"
	}
	if n%5 == 0 {
		sound += "Plang"
	}
	if n%7 == 0 {
		sound += "Plong"
	}
	if sound == "" {
		return strconv.Itoa(n)
	}
	return sound
}
