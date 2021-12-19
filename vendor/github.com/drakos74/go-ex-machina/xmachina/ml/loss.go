package ml

import (
	"fmt"
	"math"

	"github.com/drakos74/go-ex-machina/xmath"
)

// Loss defines the loss function for the evaluation of the expected and actual output.
type Loss func(expected, output xmath.Vector) xmath.Vector

// NoLoss defines a void loss function
var NoLoss Loss = func(expected, output xmath.Vector) xmath.Vector {
	return xmath.Vec(len(expected))
}

// MLoss defines the loss function for matrix inut and output.
type MLoss func(expected, output xmath.Matrix) xmath.Vector

// Diff is the simplest loss function where it s the difference of the expected abd output.
var Diff Loss = func(expected, output xmath.Vector) xmath.Vector {
	xmath.MustHaveSameSize(expected, output)
	return expected.Diff(output)
}

// Pow is the power loss function.
var Pow Loss = func(expected, output xmath.Vector) xmath.Vector {
	xmath.MustHaveSameSize(expected, output)
	return expected.Diff(output).Pow(2).Mult(0.5)
}

// CrossEntropy is the crossentropy loss function.
var CrossEntropy Loss = func(expected, output xmath.Vector) xmath.Vector {
	xmath.MustHaveSameSize(expected, output)
	return expected.Dop(func(x, y float64) float64 {
		if y == 1 {
			panic(fmt.Sprintf("cross entropy calculation threshold breached for output %v", y))
		}
		return -1 * x * math.Log(y)
	}, output)
}

// CompLoss is a compound loss function that accepts arrays.
func CompLoss(mloss Loss) MLoss {
	return func(expected, output xmath.Matrix) xmath.Vector {
		size := len(expected)
		xmath.MustHaveDim(expected, size)
		xmath.MustHaveDim(output, size)
		loss := xmath.Vec(len(expected))
		for i := 0; i < size; i++ {
			xmath.MustHaveSameSize(expected[i], output[i])
			entropy := mloss(expected[i], output[i])
			loss[i] = entropy.Sum() / float64(size)
		}
		return loss
	}
}
