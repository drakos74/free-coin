package math

import "math"

// RSI is an RSI streaming calculator
type RSI struct {
	pos   float64
	neg   float64
	count int
}

// Add adds another value for the RSI calculation and returns the intermediate result.
func (rsi *RSI) Add(f float64) int {
	if f > 0 {
		rsi.pos += f
	} else {
		rsi.neg += f
	}
	rsi.count++

	c := float64(rsi.count)

	si := (rsi.pos / c) / (rsi.neg / c)

	return int(math.Round(100 - (100 / (1 + si))))
}

// RSI calculates the RSI indicator on the given sample.
// To avoid duplicate iteration an extractor is given to use not only float slices.
func CalcRSI(values []interface{}, extractAverage func(interface{}) float64) int {

	pos := 0.0
	neg := 0.0
	count := 0
	for _, v := range values {
		f := extractAverage(v)
		if f > 0 {
			pos += f
		} else {
			neg += f
		}
		count++
	}

	c := float64(count)

	si := (pos / c) / (neg / c)

	return int(math.Round(100 - (100 / (1 + si))))

}
