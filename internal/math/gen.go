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

type Generator func(i int) float64

// GenerateFloats generates a series of floats
func GenerateFloats(num int, gen Generator) []float64 {
	ff := make([]float64, num)
	for n := 0; n < num; n++ {
		ff[n] = gen(n)
	}
	return ff
}

// VaryingSine defines a generator which varies around the given sine
func VaryingSine(base, amplitude, period float64) Generator {
	return func(i int) float64 {
		return base + amplitude*SineEvolve(i, period)
	}
}

// SineEvolve will evolve the given int to a sine
func SineEvolve(i int, p float64) float64 {
	return math.Sin(float64(i) * p)
}
