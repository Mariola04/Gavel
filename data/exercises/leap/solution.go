package solution

// IsLeapYear reports whether year is a leap year in the Gregorian calendar.
func IsLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
