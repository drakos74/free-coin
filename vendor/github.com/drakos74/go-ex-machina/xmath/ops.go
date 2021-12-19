package xmath

import (
	"fmt"
	"math"
)

// pp is the print precision for floats
const pp = 8

// Check checks if the given number is a valid one.
func Check(v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		panic(fmt.Sprintf("%v is not a valid number", v))
	}
}

// Op is a general mathematical operation from one number to another
type Op func(x float64) float64

// Round defines a round operation for the amount of digits after the comma, provided.
func Round(digits int) Op {
	factor := math.Pow(10, float64(digits))
	return func(x float64) float64 {
		return math.Round(factor*x) / factor
	}
}

// Clip clips the given number to the corresponding min or max value.
func Clip(min, max float64) Op {
	return func(x float64) float64 {
		if x < min {
			return min
		}
		if x > max {
			return max
		}
		return x
	}
}

// Scale scales the given number according to the scaling factor provided.
func Scale(s float64) Op {
	return func(x float64) float64 {
		return x * s
	}
}

// Add adds the given number to the argument.
func Add(c float64) Op {
	return func(x float64) float64 {
		return x + c
	}
}

// Unit is a predefined operation that always returns 1.
var Unit Op = func(x float64) float64 {
	return 1
}

// Sqrt returns the square root of the argument.
var Sqrt Op = func(x float64) float64 {
	return math.Sqrt(x)
}

// Square returns the square of the argument.
var Square Op = func(x float64) float64 {
	return math.Pow(x, 2)
}

// Dop is a general mathematical operation from 2 numbers to another
type Dop func(x, y float64) float64

// Mult multiplies the two numbers.
var Mult Dop = func(x, y float64) float64 {
	return x * y
}

// Div divides the given numbers.
var Div Dop = func(x, y float64) float64 {
	return x / (y + 1e-8)
}

// Diff returns the difference of the two numbers.
var Diff Dop = func(x, y float64) float64 {
	return x - y
}
