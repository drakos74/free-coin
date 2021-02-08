package math

import (
	"math"
	"strconv"
)

// TODO : format based on the value
func Format(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func Order10(f float64) int {
	log10 := math.Log10(math.Abs(f))
	return int(math.Abs(log10))
}
