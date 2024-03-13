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

// O2 returns the binary order of the value on a decimal basis
// NOTE : this does not differentiate between values bigger or smaller than 1
func O2(f float64) int {
	log2 := math.Log(math.Abs(f))
	return int(math.Abs(log2))
}

// NOTE The below try to unify the results, so that we can assess them in the same manner,
// no matter if we use a base 10 or base 2 logarithm
// see related tests TestIO10 and TestIO2

// IO10 returns the order of the value on a decimal basis
// NOTE : this does not differentiate between values bigger or smaller than 1
// NOTE : It s effectively a value inverse of the O10
// NOTE : contradictory to O10 this will differentiate based on the sign
// NOTE : it returns results in a range of 10, with 10 representing the largest absolute value !!!
func IO10(f float64) int {
	sign := 1
	if math.Signbit(f) {
		sign = -1
	}
	o := O10(f)
	return 2 * sign * invScale(5, o)
}

// IO2 returns the order of the value on a binary basis
// NOTE : this does not differentiate between values bigger or smaller than 1
// NOTE : It s effectively a value inverse of the O2
// NOTE : contradictory to O2 this will differentiate based on the sign
// NOTE : it returns results in a range of 10, with 10 representing the largest absolute value !!!
func IO2(f float64) int {
	sign := 1
	if math.Signbit(f) {
		sign = -1
	}
	o := O2(f)
	return sign * invScale(10, o)
}

// invScale inverses the value based on the given median
func invScale(m, i int) int {
	n := float64(m) / 2
	d := float64(i) - n
	return int(n - d)
}

func ToInt(ff []float64) []int {
	ii := make([]int, len(ff))
	for i, f := range ff {
		ii[i] = int(f)
	}
	return ii
}

func ToFloat(ii []int) []float64 {
	ff := make([]float64, len(ii))
	for f, i := range ii {
		ff[f] = float64(i)
	}
	return ff
}
