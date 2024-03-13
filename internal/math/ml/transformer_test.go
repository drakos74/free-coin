package ml

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"
)

func TestTransformer(t *testing.T) {
	// Create a new Transformer model
	model := NewTransformer(5, 2, 5, 25, 50)

	// Generate some training data
	trainingData, sampleData := GenerateTrainingData(5, 5)

	//// Training loop
	//numEpochs := 10
	//batchSize := 32
	//learningRate := 0.001
	//// Mini-batch training
	//for i := 0; i < len(inputs); i += batchSize {
	//	// Extract mini-batch
	//	end := i + batchSize
	//	if end > len(inputs) {
	//		end = len(inputs)
	//	}
	//	batchInputs := inputs[i:end]
	//	batchTargets := targets[i:end]
	//
	//	// Forward propagation
	//	outputs := transformer.Forward(batchInputs)
	//
	//	// Compute loss
	//	loss := computeLoss(outputs, batchTargets)
	//
	//	// Backpropagation
	//	transformer.Backpropagate(loss, learningRate)
	//}

	// Train the model
	model.Train(0.001, 1000, trainingData, sampleData)

	// Test the trained model
	inputSequence := [][]float64{{0.3, 0.1, 0.7, 0.2, 0.8}, {0.5, 0.2, 0.6, 0.1, 0.5}, {0.1, 0.6, 0.3, 0.9, 0.11}}
	fmt.Printf("inputSequence = %+v\n", inputSequence)
	translatedSequence := model.Forward(inputSequence)

	// Print the translated sequence
	fmt.Println("Translated Sequence:", translatedSequence)
}

// generateTrainingData generates training data for the sequence-to-sequence task
func generateTrainingData(numSamples int) [][]float64 {
	data := make([][]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		input := generateRandomSequence(5) // Generate a random input sequence of length 5
		output := make([]float64, len(input))
		for j, val := range input {
			output[j] = val + 5 // Add 5 to each element of the input sequence to get the output sequence
		}
		data[i] = input
	}
	return data
}

// generateRandomSequence generates a random sequence of integers of the specified length
func generateRandomSequence(length int) []float64 {
	sequence := make([]float64, length)
	for i := 0; i < length; i++ {
		sequence[i] = rand.Float64()
	}
	return sequence
}

// GenerateTrainingData generates training data for a sequence-to-sequence task
func GenerateTrainingData(numSamples, seqLength int) ([][]float64, [][]float64) {
	rand.Seed(time.Now().UnixNano())

	inputs := make([][]float64, numSamples)
	targets := make([][]float64, numSamples)

	for i := 0; i < numSamples; i++ {
		input := make([]float64, seqLength)
		for j := 0; j < seqLength; j++ {
			input[j] = rand.Float64() // Generate random integers between 0 and 99
		}

		target := make([]float64, len(input))
		copy(target, input)
		sort.Float64s(target)

		inputs[i] = input
		targets[i] = target
	}

	return inputs, targets
}
