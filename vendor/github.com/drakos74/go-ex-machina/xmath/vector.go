package xmath

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Vop is generic operation that returns a vector out of a number.
type Vop func(x float64) Vector

// VecOp is a generic operation on a vector that returns another vector.
type VecOp func(x Vector) Vector

// Unary is a unary vector operation e.g. it leaves the initial vector untouched.
var Unary VecOp = func(x Vector) Vector {
	return x
}

// Vector is an alias for a one dimensional array.
type Vector []float64

// Vec creates a new vector.
func Vec(dim int) Vector {
	v := make([]float64, dim)
	return v
}

// Vector sanity check methods

// Check checks if the elements of the vector are well defined
func (v Vector) Check() {
	for _, vv := range v {
		Check(vv)
	}
}

// String prints the vector in an easily readable form
func (v Vector) String() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("(%d)", len(v)))
	builder.WriteString("[ ")
	for i := 0; i < len(v); i++ {
		ss := ""
		if v[i] > 0 {
			ss = " "
		}
		builder.WriteString(fmt.Sprintf("%s%s", ss, strconv.FormatFloat(v[i], 'f', pp, 64)))
		if i < len(v)-1 {
			// dont add the comma to the last element
			builder.WriteString(" , ")
		}
	}
	builder.WriteString(" ]")
	return builder.String()
}

// Vector Operations

// Dot returns the dot product of the 2 vectors
func (v Vector) Dot(w Vector) float64 {
	MustHaveSameSize(v, w)
	var p float64
	for i := 0; i < len(v); i++ {
		p += v[i] * w[i]
	}
	return p
}

// Prod returns the product of the given vectors
// it returns a matrix
func (v Vector) Prod(w Vector) Matrix {
	z := Mat(len(v)).Of(len(w))
	for i := 0; i < len(v); i++ {
		for j := 0; j < len(w); j++ {
			z[i][j] = v[i] * w[j]
		}
	}
	return z
}

// X returns the hadamard product of the given vectors.
// e.g. point-wise multiplication
func (v Vector) X(w Vector) Vector {
	MustHaveSameSize(v, w)
	z := Vec(len(v))
	for i := 0; i < len(v); i++ {
		z[i] = v[i] * w[i]
	}
	return z
}

// Stack concatenates 2 vectors , producing another with the sum of their lengths.
func (v Vector) Stack(w Vector) Vector {
	x := Vec(len(v) + len(w))
	return x.With(append(v, w...)...)
}

// Add adds 2 vectors.
func (v Vector) Add(w Vector) Vector {
	MustHaveSameSize(v, w)
	z := Vec(len(v))
	for i := 0; i < len(v); i++ {
		z[i] = v[i] + w[i]
	}
	return z
}

// Diff returns the difference of the corresponding elements between the given vectors
func (v Vector) Diff(w Vector) Vector {
	return v.Dop(Diff, w)
}

// Pow returns a vector with all the elements to the given power
func (v Vector) Pow(p float64) Vector {
	return v.Op(func(x float64) float64 {
		return math.Pow(x, p)
	})
}

// Mult multiplies a vector with a constant number
func (v Vector) Mult(s float64) Vector {
	return v.Op(func(x float64) float64 {
		return x * s
	})
}

// Round rounds all elements of the given vector
func (v Vector) Round() Vector {
	return v.Op(math.Round)
}

// Sum returns the sum of all elements of the vector
func (v Vector) Sum() float64 {
	var sum float64
	for i := 0; i < len(v); i++ {
		sum += v[i]
	}
	return sum
}

// Norm returns the norm of the vector
func (v Vector) Norm() float64 {
	var sum float64
	for i := 0; i < len(v); i++ {
		sum += math.Pow(v[i], 2)
	}
	return math.Sqrt(sum)
}

// Copy copies the vector into a new one with the same values
// this is for cases where we want to apply mutations, but would like to leave the initial vector intact
func (v Vector) Copy() Vector {
	w := Vec(len(v))
	for i := 0; i < len(v); i++ {
		w[i] = v[i]
	}
	return w
}

// Vector operation abstractions

// Op applies to each of the elements a specific function
func (v Vector) Op(transform Op) Vector {
	w := Vec(len(v))
	for i := range v {
		w[i] = transform(v[i])
	}
	return w
}

// Dop applies to each of the elements a specific function based on the elements index
func (v Vector) Dop(transform Dop, w Vector) Vector {
	z := Vec(len(v))
	for i := range v {
		z[i] = transform(v[i], w[i])
	}
	return z
}

// Vop defines a dual operation on the vector, where first we iterate over all elemenets in order to
func (v Vector) Vop(dop Dop, vop Vop) Matrix {
	// we want to make a pairwise vector out of the initial vector and apply a transform to a matrix
	l := len(v) - 1
	mat := Mat(l)
	j := 0
	for i := l; i > 0; i-- {
		diff := dop(v[i], v[i-1])
		j++
		index := l - j
		mat[index] = vop(diff)
	}
	return mat
}

// Vector Construction methods

// With applies the given elements in the corresponding positions of the vector
func (v Vector) With(w ...float64) Vector {
	MustHaveSameSize(v, w)
	for i, vv := range w {
		v[i] = vv
	}
	return v
}

// Generate generates values for the vector
func (v Vector) Generate(gen VectorGenerator) Vector {
	return gen(len(v), 0)
}

// VectorGenerator is a type alias defining the creation instructions for vectors
// s is the size of the vector
type VectorGenerator func(s, index int) Vector

// VoidVector creates a vector with zeros
var VoidVector VectorGenerator = func(s, index int) Vector {
	return Vec(s)
}

// Row defines a vector at the corresponding row index of a matrix
var Row = func(m ...Vector) VectorGenerator {
	return func(s, index int) Vector {
		MustHaveSize(m[index], s)
		return m[index]
	}
}

// Rand generates a vector of the given size with random values between min and max
// op defines a scaling operation for the min and max, based on the size of the vector
var Rand = func(min, max float64, op Op) VectorGenerator {
	rand.Seed(time.Now().UnixNano())
	return func(p, index int) Vector {
		mmin := min / op(float64(p))
		mmax := max / op(float64(p))
		w := Vec(p)
		for i := 0; i < p; i++ {
			w[i] = rand.Float64()*(mmax-mmin) + mmin
		}
		return w
	}
}

// Const generates a vector of the given size with constant values
var Const = func(v float64) VectorGenerator {
	return func(p, index int) Vector {
		w := Vec(p)
		for i := 0; i < p; i++ {
			w[i] = v
		}
		return w
	}
}

// ScaledVectorGenerator produces a vector generator scaled by the given factor
type ScaledVectorGenerator func(d float64) VectorGenerator

// RangeSqrt produces a vector generator scaled by the given factor
// and within the range provided
var RangeSqrt = func(min, max float64) ScaledVectorGenerator {
	return func(d float64) VectorGenerator {
		return Rand(min, max, func(x float64) float64 {
			return math.Sqrt(d * x)
		})
	}
}

// Range produces a vector generator scaled by the given factor
// and within the range provided
var Range = func(min, max float64) ScaledVectorGenerator {
	return func(d float64) VectorGenerator {
		return Rand(min, max, func(x float64) float64 {
			return x
		})
	}
}

// MustHaveSize will check and make sure that the given vector is of the given size
func MustHaveSize(v Vector, n int) {
	if len(v) != n {
		panic(fmt.Sprintf("vector must have size '%v' vs '%v'", v, n))
	}
}

// MustHaveSameSize verifies if the given vectors are of the same size
func MustHaveSameSize(v, w Vector) {
	if len(v) != len(w) {
		panic(fmt.Sprintf("vectors must have the same size '%v' vs '%v'", len(v), len(w)))
	}
}

// Vector related operations
var UpOrDown Vop = func(x float64) Vector {
	v := Vec(2)
	if x > 0 {
		v[0] = 1
	} else if x < 0 {
		v[1] = 0
	}
	return v
}
