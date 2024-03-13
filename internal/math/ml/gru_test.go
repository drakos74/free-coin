package ml

import (
	"fmt"
	"math"
	"testing"
)

func TestOnSinus(t *testing.T) {

	inputSize := 1
	hiddenSize := 64
	outputSize := 1

	// Creating GRU model
	gru := NewGRU(inputSize, hiddenSize, outputSize)

	// Generate sinusoidal data
	sinData := generateSinData(100)

	in := sinData[:len(sinData)-1]
	out := sinData[1:]

	epochs, err := gru.Train(0.1, 0.1, 1000, in, out)
	fmt.Printf("epochs = %+v\n", epochs)
	fmt.Printf("err = %+v\n", err)

	// Testing
	var predictions []float64
	prevHidden := make([]float64, hiddenSize)
	predictionErr := 0.0
	for i := 0; i < len(sinData); i++ {
		input := sinData[i]
		output, hidden := gru.Step(input, prevHidden)
		if i < len(sinData)-1 {
			predictionErr += math.Abs(sinData[i+1][0] - output[0])
		}
		predictions = append(predictions, output[0])
		prevHidden = hidden
	}

	// Displaying results
	fmt.Println("Actual sinusoidal data:", sinData)
	fmt.Println("Predicted sinusoidal data:", predictions)
	fmt.Printf("predictionErr = %+v\n", predictionErr)
}

func generateSinData(steps int) [][]float64 {
	data := make([][]float64, steps)
	for i := 0; i < steps; i++ {
		data[i] = []float64{math.Sin(float64(i) * 0.1)} // Generating sinusoidal data
	}
	return data
}
