package net

import (
	"github.com/drakos74/go-ex-machina/xmath"
)

// Op represents a generic operation on a vector.
type Op interface {
	Fwd(x xmath.Vector) xmath.Vector
	Bwd(dy xmath.Vector) xmath.Vector
}

// BiOp represents a generic operation on two vectors producing one vector as output.
type BiOp interface {
	Fwd(a, b xmath.Vector) xmath.Vector
	Bwd(dc xmath.Vector) (da, db xmath.Vector)
}

// MulCell represents a BiOp cell that applies point-wise multiplication.
type MulCell struct {
	a, b xmath.Vector
}

// NewMulCell creates a new mul cell.
func NewMulCell() *MulCell {
	return &MulCell{}
}

// Fwd applies the operation.
func (m *MulCell) Fwd(a, b xmath.Vector) xmath.Vector {
	m.a = a
	m.b = b
	return a.X(b)
}

// Bwd back-propagates the error.
func (m *MulCell) Bwd(dc xmath.Vector) (da, db xmath.Vector) {
	return dc.X(m.b), dc.X(m.a)
}

// StackCell represents a BiOp tht stacks the two vectors.
type StackCell struct {
	border int
}

// NewStackCell creates a new stack cell.
func NewStackCell(border int) *StackCell {
	return &StackCell{
		border: border,
	}
}

// Fwd applies the operation.
func (s *StackCell) Fwd(a, b xmath.Vector) xmath.Vector {
	return a.Stack(b)
}

// Bwd back-propagates the error.
func (s *StackCell) Bwd(dc xmath.Vector) (da, db xmath.Vector) {
	return dc[:s.border], dc[s.border:]
}
