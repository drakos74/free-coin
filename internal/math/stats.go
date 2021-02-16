package math

import (
	"math"
)

// RSI is an RSI streaming calculator
type RSI struct {
	pos      float64
	neg      float64
	countPos int
	countNeg int
}

// Add adds another value for the RSI calculation and returns the intermediate result.
func (rsi *RSI) Add(f float64) (value, count int) {
	if f > 0 {
		rsi.pos += f
		rsi.countPos++
	} else {
		rsi.neg += math.Abs(f)
		rsi.countNeg++
	}

	return rsi.Get()
}

// Get returns the rsi for the added samples
func (rsi *RSI) Get() (value, count int) {

	si := (rsi.pos / float64(rsi.countPos)) / (rsi.neg / float64(rsi.countNeg))

	return int(math.Round(100 - (100 / (1 + si)))), rsi.countPos + rsi.countNeg
}
