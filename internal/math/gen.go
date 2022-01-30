package math

import "math"

func Series(factor float64, limit int) []float64 {
	xx := make([]float64, 0)
	for i := 0; i < limit; i++ {
		xx = append(xx, factor*float64(i))
	}
	return xx
}

func Sine(factor float64, limit int, v float64) []float64 {
	xx := make([]float64, 0)
	for i := 0; i < limit; i++ {
		xx = append(xx, factor*SineEvolve(i, v))
	}
	return xx
}

func SineEvolve(i int, p float64) float64 {
	return math.Sin(float64(i) * p)
}
