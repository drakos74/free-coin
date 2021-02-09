package math

import (
	"math"
	"strconv"
)

// Format formats a float based on the given precision
// TODO : format based on the value
func Format(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// O10 returns the order of the value on a decimal basis
// NOTE : this does not differentiate between values bigger or smaller than 1
func O10(f float64) int {
	log10 := math.Log10(math.Abs(f))
	return int(math.Abs(log10))
}
