package ml

import (
	"math"

	"github.com/drakos74/go-ex-machina/xmath"
)

// Activation defines the activation function for an ml module.
type Activation interface {
	F(x float64) float64
	D(x float64) float64
}

// Sigmoid uses sigmoid activation.
var Sigmoid = sigmoid{}

type sigmoid struct {
}

// F applies the activation function.
func (s sigmoid) F(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-1*x))
}

// D returns the derivative of the activation function.
func (s sigmoid) D(y float64) float64 {
	return y * (1.0 - y)
}

// TanH defines the tanh activation function.
var TanH = tanH{}

type tanH struct {
}

// F applies the activation function.
func (t tanH) F(x float64) float64 {
	return math.Tanh(x)
}

// D returns the derivative of the activation function.
func (t tanH) D(y float64) float64 {
	return 1 - math.Pow(y, 2)
}

// ReLU defines the relu activation function.
var ReLU = relu{}

type relu struct {
}

// F applies the activation function.
func (r relu) F(x float64) float64 {
	return math.Max(0, x)
}

// D returns the derivative of the activation function.
func (r relu) D(x float64) float64 {
	return 1
}

// Void defines no activation at all
type Void struct {
}

// F applies the activation function.
func (v Void) F(x float64) float64 {
	return x
}

// D returns the derivative of the activation function.
// TODO : investigate if it should be 1 or something else.
func (v Void) D(x float64) float64 {
	return x
}

// SoftActivation defines a vector based activation function.
type SoftActivation interface {
	F(v xmath.Vector) xmath.Vector
	D(s xmath.Vector) xmath.Matrix
}

// SoftUnary is a unary activation.
type SoftUnary struct {
}

// F applies the activation function.
func (s SoftUnary) F(v xmath.Vector) xmath.Vector {
	return v
}

// D returns the derivative of the activation function.
func (s SoftUnary) D(y xmath.Vector) xmath.Matrix {
	return xmath.Mat(len(y)).From(y)
}

// SoftMax defines the softmax activation function.
type SoftMax struct {
}

func (sm SoftMax) max(v []float64) float64 {
	var max float64
	for _, x := range v {
		max = math.Max(x, max)
	}
	return max
}

func (sm SoftMax) expSum(v []float64, max float64) float64 {
	var sum float64
	for _, x := range v {
		sum += sm.exp(x, max)
	}
	return sum
}

func (sm SoftMax) exp(x, max float64) float64 {
	return math.Exp(x - max)
}

// F applies the activation function.
func (sm SoftMax) F(v xmath.Vector) xmath.Vector {
	softmax := xmath.Vec(len(v))
	max := sm.max(v)
	sum := sm.expSum(v, max)
	for i, x := range v {
		softmax[i] = sm.exp(x, max) / sum
	}
	return softmax
}

// D returns the derivative of the activation function.
func (sm SoftMax) D(s xmath.Vector) xmath.Matrix {
	jacobian := xmath.Diag(s)
	for i := range jacobian {
		for j := range jacobian[i] {
			if i == j {
				jacobian[i][j] = s[i] * (1 - s[i])
			} else {
				jacobian[i][j] = -s[i] * s[j]
			}
		}
	}
	return jacobian
}
