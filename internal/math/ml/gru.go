package ml

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/drakos74/go-ex-machina/xmath"
)

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func sigmoidDerivative(x float64) float64 {
	return sigmoid(x) * (1 - sigmoid(x))
}

func apply(f func(x float64) float64) func([]float64) []float64 {
	return func(vec []float64) []float64 {
		result := make([]float64, len(vec))
		for i, v := range vec {
			result[i] = f(v)
		}
		return result
	}
}

type GRU struct {
	inputSize     int
	hiddenSize    int
	outputSize    int
	wr            [][]float64 // reset gate weights
	ur            [][]float64 // reset gate biases
	wz            [][]float64 // update gate weights
	uz            [][]float64 // update gate biases
	wx            [][]float64 // candidate weights
	ux            [][]float64 // candidate biases
	wo            [][]float64 // New weight matrix
	bo            []float64   // New bias vector
	resetGate     []float64
	updateGate    []float64
	combinedInput []float64
}

func NewGRU(inputSize, hiddenSize, outputSize int) *GRU {
	rand.Seed(time.Now().UnixNano())

	gru := &GRU{
		inputSize:     inputSize,
		hiddenSize:    hiddenSize,
		outputSize:    outputSize,
		wr:            heMat(hiddenSize, inputSize),
		ur:            heMat(hiddenSize, hiddenSize),
		wz:            heMat(hiddenSize, inputSize),
		uz:            heMat(hiddenSize, hiddenSize),
		wx:            heMat(hiddenSize, inputSize),
		ux:            heMat(hiddenSize, hiddenSize),
		wo:            heMat(outputSize, hiddenSize), // New weight matrix
		bo:            randVec(outputSize),
		resetGate:     randVec(hiddenSize),
		updateGate:    randVec(hiddenSize),
		combinedInput: randVec(hiddenSize),
	}

	return gru
}

func randVec(cols int) []float64 {
	vec := make([]float64, cols)
	for i := range vec {
		vec[i] = rand.Float64()*2 - 1 // range [-1,1]
	}
	return vec
}

func randMat(rows, cols int) [][]float64 {
	mat := make([][]float64, rows)
	for i := range mat {
		mat[i] = make([]float64, cols)
		for j := range mat[i] {
			mat[i][j] = rand.Float64()*2 - 1 // range [-1,1]
		}
	}
	return mat
}

func (gru *GRU) Step(input []float64, prevHidden []float64) ([]float64, []float64) {
	gru.resetGate = apply(sigmoid)(addVec(dotMat(gru.wr, input), dotMat(gru.ur, prevHidden)))
	gru.updateGate = apply(sigmoid)(addVec(dotMat(gru.wz, input), dotMat(gru.uz, prevHidden)))
	gru.combinedInput = tanh(addVec(dotMat(gru.wx, input), hadamardVec(gru.resetGate, dotMat(gru.ux, prevHidden))))

	hidden := addVec(hadamardVec(gru.updateGate, prevHidden), hadamardVec(subVec(make([]float64, len(prevHidden)), gru.updateGate), gru.combinedInput))

	// Calculate the output
	output := tanh(addVec(dotMat(gru.wo, hidden), gru.bo))

	return output, hidden
}

// Train trains the gru network,
// NOTE : we assume in and out has the same size
func (gru *GRU) Train(learningRate float64, threshold float64, maxEpochs int, in, out [][]float64) (epochs int, err []float64) {
	// Training
	globalErr := make([]float64, len(in))
	// we ad-hoc assume here that length of input is larger than the length of the output
	for epoch := 0; epoch < maxEpochs; epoch++ {
		prevHidden := randVec(gru.hiddenSize)
		globalErr = make([]float64, len(in))
		// for this network set up, we can only train on the output size
		for i := 0; i < len(in); i++ {
			input := in[i]
			target := out[i]

			// Step forward to compute the current output and hidden state
			output, hidden := gru.Step(input, prevHidden)

			// Compute the error between the predicted output and the target output
			outputError := subVec(output, target)
			globalErr[i] = dotVec(outputError, outputError)

			// Calculate the gradient for the output layer
			outputGradient := hadamardVec(outputError, tanhDerivative(output))
			checkVecGradient(outputGradient)
			ClipVecByNorm(outputGradient, 1)

			// Calculate the gradients for the output layer weights and biases
			woGradient := outerProduct(outputGradient, hidden)
			checkMatGradient(woGradient)
			ClipMatByNorm(woGradient, 1)

			boGradient := outputGradient
			checkVecGradient(boGradient)
			ClipVecByNorm(boGradient, 1)

			// Backpropagate through the output layer to get gradients for hidden layer
			hiddenError := dotMat(transpose(gru.wo), outputGradient)

			// Calculate the gradient for the combined input of the hidden layer
			combinedInputGradient := hadamardVec(hiddenError, tanhDerivative(gru.combinedInput))
			checkVecGradient(combinedInputGradient)
			ClipVecByNorm(combinedInputGradient, 1)

			// Calculate the gradient for the update gate
			updateGateGradient := outerProduct(hidden, combinedInputGradient)
			checkMatGradient(updateGateGradient)
			ClipMatByNorm(updateGateGradient, 1)
			updateGateBiasGradient := updateGateGradient

			// Calculate the gradient for the reset gate
			resetGateGradient := outerProduct(prevHidden, combinedInputGradient)
			checkMatGradient(resetGateGradient)
			ClipMatByNorm(resetGateGradient, 1)
			resetGateBiasGradient := resetGateGradient

			// Calculate the gradients for the candidate weights and biases
			wxGradient := outerProduct(combinedInputGradient, input)
			checkMatGradient(wxGradient)
			ClipMatByNorm(wxGradient, 1)

			uxGradient := outerProduct(combinedInputGradient, prevHidden)
			checkMatGradient(uxGradient)
			ClipMatByNorm(uxGradient, 1)

			// Update the weights and biases using the gradients and the learning rate
			gru.wo = addMat(gru.wo, scalarMatMul(-1*learningRate, woGradient))
			gru.bo = addVec(gru.bo, scalarVecMul(-1*learningRate, boGradient))

			gru.wx = addMat(gru.wx, scalarMatMul(-1*learningRate, wxGradient))
			gru.ux = addMat(gru.ux, scalarMatMul(-1*learningRate, uxGradient))

			gru.wz = addMat(gru.wz, scalarMatMul(-1*learningRate, updateGateGradient))
			gru.uz = addMat(gru.uz, scalarMatMul(-1*learningRate, updateGateBiasGradient))

			gru.wr = addMat(gru.wr, scalarMatMul(-1*learningRate, resetGateGradient))
			gru.ur = addMat(gru.ur, scalarMatMul(-1*learningRate, resetGateBiasGradient))
		}
		if xmath.Vec(len(in)).With(globalErr...).Norm() < threshold {
			return epoch, globalErr
		}
	}
	return maxEpochs, globalErr
}

func dotMat(a [][]float64, b []float64) []float64 {
	res := make([]float64, len(a))
	for i := range a {
		for j := range a[i] {
			res[i] += a[i][j] * b[j]
		}
	}
	return res
}

func dotVec(a, b []float64) float64 {
	res := 0.0
	for i := range a {
		res += a[i] * b[i]
	}
	return res
}

func addVec(a, b []float64) []float64 {
	res := make([]float64, len(a))
	for i := range a {
		res[i] = a[i] + b[i]
	}
	return res
}

// This function takes two matrices a and b, represented as slices of slices of float64 values,
// and returns their element-wise sum as a new matrix.
// It assumes that both matrices have the same number of rows and columns.
// Each element of the resulting matrix is the sum of the corresponding elements from matrices a and b.
func addMat(a, b [][]float64) [][]float64 {
	rows := len(a)
	cols := len(a[0])
	res := make([][]float64, rows)
	for i := 0; i < rows; i++ {
		res[i] = make([]float64, cols)
		for j := 0; j < cols; j++ {
			res[i][j] = a[i][j] + b[i][j]
		}
	}
	return res
}

func subVec(a []float64, b []float64) []float64 {
	res := make([]float64, len(a))
	for i := range a {
		res[i] = a[i] - b[i]
	}
	return res
}

// This function takes a matrix mat as input and computes its transpose, resulting in a new matrix
// where the rows of the input matrix become the columns of the output matrix and vice versa.
func transpose(mat [][]float64) [][]float64 {
	rows := len(mat)
	cols := len(mat[0])
	transposed := make([][]float64, cols)
	for j := range transposed {
		transposed[j] = make([]float64, rows)
		for i := range transposed[j] {
			transposed[j][i] = mat[i][j]
		}
	}
	return transposed
}

// This function takes a scalar value and a matrix represented as a slice of slices of float64 values,
// and it returns the result of multiplying the matrix by the scalar.
// Each element of the input matrix is multiplied by the scalar to produce
// the corresponding element of the result matrix.
func scalarMatMul(scalar float64, mat [][]float64) [][]float64 {
	res := make([][]float64, len(mat))
	for i := range mat {
		res[i] = make([]float64, len(mat[i]))
		for j := range mat[i] {
			res[i][j] = scalar * mat[i][j]
		}
	}
	return res
}

// This function takes a scalar value and a vector represented as a slice of float64 values,
// and it returns the result of multiplying the vector by the scalar.
func scalarVecMul(scalar float64, vec []float64) []float64 {
	res := make([]float64, len(vec))
	for i := range vec {
		res[i] = scalar * vec[i]
	}
	return res
}

// This function takes two vectors vec1 and vec2 as input and computes their outer product,
// resulting in a matrix where each element is the product of the corresponding elements of the input vectors.
// The resulting matrix has dimensions (len(vec1), len(vec2)).
//
// You can use this outerProduct function in the backpropagation logic provided earlier
// to calculate the gradient for the weights between the hidden and output layers.
func outerProduct(vec1 []float64, vec2 []float64) [][]float64 {
	rows := len(vec1)
	cols := len(vec2)
	result := make([][]float64, rows)
	for i := range result {
		result[i] = make([]float64, cols)
		for j := range result[i] {
			result[i][j] = vec1[i] * vec2[j]
		}
	}
	return result
}

func hadamardVec(a, b []float64) []float64 {
	res := make([]float64, len(a))
	for i := range a {
		res[i] = a[i] * b[i]
	}
	return res
}

func tanh(x []float64) []float64 {
	res := make([]float64, len(x))
	for i := range x {
		res[i] = math.Tanh(x[i])
	}
	return res
}

// This function takes a slice of float64 values as input and returns a slice of the same length
// containing the derivatives of the tanh function with respect to each input value.
func tanhDerivative(x []float64) []float64 {
	res := make([]float64, len(x))
	for i := range x {
		res[i] = 1.0 - math.Pow(math.Tanh(x[i]), 2) // Derivative of tanh
	}
	return res
}

var check_gradients = false

func checkVecGradient(dg []float64) {
	if !check_gradients {
		return
	}
	for _, g := range dg {
		v := math.Abs(g)
		if v < 0.0001 {
			fmt.Printf("vanishing gradient !!! dg = %+v\n", dg)
			return
		} else if v > 10 {
			fmt.Printf("exploding gradient !!! dg = %+v\n", dg)
			return
		}
	}
}

func checkMatGradient(dg [][]float64) {
	if !check_gradients {
		return
	}
	for _, gg := range dg {
		for _, g := range gg {
			v := math.Abs(g)
			if v < 0.0001 {
				fmt.Printf("vanishing gradient !!! dg = %+v\n", dg)
				return
			} else if v > 10 {
				fmt.Printf("exploding gradient !!! dg = %+v\n", dg)
				return
			}
		}
	}
}

// ClipMatByNorm clips gradients based on their global L2 norm.
func ClipMatByNorm(gradients [][]float64, maxNorm float64) {
	// Calculate the global L2 norm of gradients
	totalNorm := 0.0
	for _, grad := range gradients {
		for _, val := range grad {
			totalNorm += val * val
		}
	}
	totalNorm = math.Sqrt(totalNorm)

	// If the total norm exceeds the maximum allowed norm, scale down gradients
	if totalNorm > maxNorm {
		scale := maxNorm / totalNorm
		for _, grad := range gradients {
			for i := range grad {
				grad[i] *= scale
			}
		}
	}
}

// ClipVecByNorm clips gradients based on their global L2 norm.
func ClipVecByNorm(gradients []float64, maxNorm float64) {
	// Calculate the global L2 norm of gradients
	totalNorm := 0.0
	for _, grad := range gradients {
		totalNorm += grad * grad
	}
	totalNorm = math.Sqrt(totalNorm)

	// If the total norm exceeds the maximum allowed norm, scale down gradients
	if totalNorm > maxNorm {
		scale := maxNorm / totalNorm
		for i, _ := range gradients {
			gradients[i] *= scale
		}
	}
}

func heMat(n, m int) [][]float64 {
	rand.Seed(time.Now().UnixNano())
	w := make([][]float64, n)
	for i := range w {
		w[i] = make([]float64, m)
		for j := range w[i] {
			scale := math.Sqrt(2.0 / float64(m))
			w[i][j] = rand.NormFloat64() * scale
		}
	}
	return w
}
