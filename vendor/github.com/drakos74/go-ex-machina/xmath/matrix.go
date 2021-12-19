package xmath

import (
	"fmt"
	"strings"
)

type Matrix []Vector

// Diag creates a new diagonal Matrix with the given elements in the diagonal
func Diag(v Vector) Matrix {
	m := Mat(len(v))
	for i := range v {
		m[i] = Vec(len(v))
		m[i][i] = v[i]
	}
	return m
}

// Mat creates a newMatrix of the given dimension
func Mat(m int) Matrix {
	mat := make([]Vector, m)
	return mat
}

// T calculates the transpose of a matrix
func (m Matrix) T() Matrix {
	n := Mat(len(m[0])).Of(len(m))
	for i := range m {
		for j := range m[i] {
			n[j][i] = m[i][j]
		}
	}
	return n
}

// Sum returns a vector that carries the sum of all elements for each row of the Matrix
func (m Matrix) Sum() Vector {
	v := Vec(len(m))
	for i := range m {
		v[i] = m[i].Sum()
	}
	return v
}

// Add returns the addition operation on 2 matrices
func (m Matrix) Add(v Matrix) Matrix {
	w := Mat(len(m))
	for i := range m {
		n := Vec(len(m[i]))
		for j := 0; j < len(m[i]); j++ {
			n[j] = m[i][j] + v[i][j]
		}
		w[i] = n
	}
	return w
}

// Dot returns the product of the given matrix with the matrix
func (m Matrix) Dot(v Matrix) Matrix {
	w := Mat(len(m))
	for i := range m {
		for j := 0; j < len(v); j++ {
			MustHaveSameSize(m[i], v[j])
			w[i][j] = m[i].Dot(v[j])
		}
	}
	return w
}

// Prod returns the cross product of the given vector with the matrix
func (m Matrix) Prod(v Vector) Vector {
	w := Vec(len(m))
	for i := range m {
		MustHaveSameSize(m[i], v)
		w[i] = m[i].Dot(v)
	}
	return w
}

// Mult multiplies each element of the matrix with the given factor
func (m Matrix) Mult(s float64) Matrix {
	n := Mat(len(m))
	for i := range m {
		n[i] = m[i].Mult(s)
	}
	return n
}

// Of initialises the rows of the matrix with vectors of the given length
func (m Matrix) Of(n int) Matrix {
	for i := 0; i < len(m); i++ {
		m[i] = Vec(n)
	}
	return m
}

// With creates a matrix with the given vector replicated at each row
func (m Matrix) From(v Vector) Matrix {
	for i := range m {
		m[i] = v
	}
	return m
}

// With applies the elements of the given vectors to the corresponding positions in the matrix
func (m Matrix) With(v ...Vector) Matrix {
	for i := range m {
		m[i] = v[i]
	}
	return m
}

// Generate generates the rows of the matrix using the generator func
func (m Matrix) Generate(p int, gen VectorGenerator) Matrix {
	for i := range m {
		m[i] = gen(p, i)
	}
	return m
}

// Copy copies the matrix into a new one with the same values
// this is for cases where we want to apply mutations, but would like to leave the initial vector intact
func (m Matrix) Copy() Matrix {
	n := Mat(len(m))
	for i := 0; i < len(m); i++ {
		n[i] = m[i].Copy()
	}
	return n
}

// Op applies to each of the elements a specific function
func (m Matrix) Op(transform Op) Matrix {
	n := Mat(len(m))
	for i := range m {
		n[i] = m[i].Op(transform)
	}
	return n
}

// Op applies to each of the elements a specific function
func (m Matrix) Dop(transform Dop, n Matrix) Matrix {
	w := Mat(len(m))
	for i := range m {
		w[i] = m[i].Dop(transform, n[i])
	}
	return w
}

// Vop applies the corresponding Vop operation to all rows, producing a Cube
func (m Matrix) Vop(dop Dop, vop Vop) Cube {
	w := Cb(len(m))
	for i := range m {
		w[i] = m[i].Vop(dop, vop)
	}
	return w
}

// String prints the matrix in an easily readable form
func (m Matrix) String() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("(%d)", len(m)))
	builder.WriteString("\n")
	for i := 0; i < len(m); i++ {
		builder.WriteString("\t")
		builder.WriteString(fmt.Sprintf("[%d]", i))
		builder.WriteString(fmt.Sprintf("%v", m[i]))
		builder.WriteString("\n")
	}
	return builder.String()
}

// MustHaveSize will check and make sure that the given vector is of the given size
func MustHaveDim(m Matrix, n int) {
	if len(m) != n {
		panic(fmt.Sprintf("matrix must have primary dimension '%v' vs '%v'", m, n))
	}
}
