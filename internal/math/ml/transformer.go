package ml

import (
	"fmt"
	"math"

	"golang.org/x/exp/rand"
)

// Constants for the Transformer model
const (
	epsilon = 0.1
)

type Transformer struct {
	encoderLayers []*EncoderLayer
	decoderLayers []*DecoderLayer
}

func NewTransformer(numHeads, numLayers int, dim ...int) *Transformer {
	transformer := &Transformer{
		encoderLayers: make([]*EncoderLayer, numLayers),
		decoderLayers: make([]*DecoderLayer, numLayers),
	}

	for i := 0; i < numLayers; i++ {
		transformer.encoderLayers[i] = &EncoderLayer{
			selfAttention: NewMultiHeadAttention(numHeads, dim[i], dim[i]),
			feedForward: NewFeedForward(meta{
				kind:  "encode",
				dim:   []int{dim[i], dim[i+1]},
				layer: []int{i},
			}, dim[i], dim[i+1]),
		}
		ri := numLayers - i
		transformer.decoderLayers[i] = &DecoderLayer{
			selfAttention:    NewMultiHeadAttention(numHeads, dim[0], dim[0]),
			encoderAttention: NewMultiHeadAttention(numHeads, dim[ri], dim[ri]),
			feedForward: NewFeedForward(meta{
				kind:  "decode",
				dim:   []int{dim[ri], dim[ri-1]},
				layer: []int{i + numLayers},
			}, dim[ri], dim[ri-1]),
		}
	}

	return transformer
}

func (model *Transformer) Forward(inputSequence [][]float64) [][]float64 {
	// Encode input sequence
	encoderOutput := model.Encode(inputSequence)

	// Decode using encoder output
	outputSequence := model.Decode(encoderOutput, inputSequence)

	return outputSequence
}

func copyMat(m [][]float64) [][]float64 {
	n := make([][]float64, len(m))
	for i := range m {
		n[i] = make([]float64, len(m[i]))
		for j := range m[i] {
			n[i][j] = m[i][j]
		}
	}
	return n
}

func copyVec(v []float64) []float64 {
	w := make([]float64, len(v))
	for i := range v {
		w[i] = v[i]
	}
	return w
}

func (model *Transformer) Encode(input [][]float64) [][]float64 {
	// Iterate through each encoder layer
	outputSequence := copyMat(input)
	for i, layer := range model.encoderLayers {
		fmt.Printf("%d input = %+v | %+v\n", i, len(outputSequence), len(outputSequence[0]))
		outputSequence = layer.Encode(outputSequence)
	}
	fmt.Printf("encode output = %+v | %+v\n", len(outputSequence), len(outputSequence[0]))
	return outputSequence
}

func (model *Transformer) Decode(input [][]float64, encoderOutput [][]float64) [][]float64 {
	// Convert target sequence into embedding vectors (not shown here)
	output := copyMat(input)
	// Iterate through each decoder layer
	fmt.Printf("encoderOutput = %+v | %+v\n", len(encoderOutput), len(encoderOutput[0]))
	for i, layer := range model.decoderLayers {
		fmt.Printf("%d output = %+v | %+v\n", i, len(output), len(output[0]))
		if i == 0 {
			output = layer.Decode(input, encoderOutput)
		} else {
			output = layer.Decode(output, output)
		}
	}
	fmt.Printf("decode output (%d) = %+v\n", len(output), output)
	return output
}

func (model *Transformer) Train(learningRate float64, epochs int, inputSequence [][]float64, targetSequence [][]float64) {
	for epoch := 0; epoch < epochs; epoch++ {
		// Forward propagation
		encodedInput := model.Encode(inputSequence)
		output := model.Decode(inputSequence, encodedInput)
		// Compute loss
		loss := CalculateLossMatrix(output, targetSequence)

		// Backpropagation
		model.BackProp(loss, learningRate)

		if epoch%100 == 0 {
			fmt.Printf("Epoch %d, Loss: %f\n", epoch+1, loss)
		}
	}
}

// BackProp function for backpropagation
func (model *Transformer) BackProp(loss [][]float64, learningRate float64) {
	// Backpropagate through decoder layers
	for i := len(model.decoderLayers); i > 0; i-- {
		layer := model.decoderLayers[i-1]
		// Backpropagate through feedforward network
		layer.feedForward.BackProp(loss, learningRate)
		// Backpropagate through encoder-decoder attention mechanism
		layer.encoderAttention.BackProp(loss, learningRate)
		// Backpropagate through self-attention mechanism
		layer.selfAttention.BackProp(loss, learningRate)
	}

	// Backpropagate through encoder layers
	for i := len(model.encoderLayers); i > 0; i-- {
		layer := model.encoderLayers[i-1]
		// Backpropagate through feedforward network
		layer.feedForward.BackProp(loss, learningRate)
		// Backpropagate through self-attention mechanism
		layer.selfAttention.BackProp(loss, learningRate)
	}
}

type EncoderLayer struct {
	selfAttention *MultiHeadAttention
	feedForward   *FeedForward
}

// Encode applies the Transformer encoder layer to encode input sequences
func (layer *EncoderLayer) Encode(input [][]float64) [][]float64 {
	// Apply Multi-Head Attention mechanism
	selfAttentionOutput := layer.selfAttention.Encode(input)

	// Apply add-and-normalize layer
	selfAttentionOutput = NormalizeMat(ResidualConnectionMat(input, selfAttentionOutput))

	// Apply FeedForward layer
	feedForwardOutput := layer.feedForward.Encode(selfAttentionOutput)

	// Apply add-and-normalize layer
	output := NormalizeMat(ResidualConnectionMat(selfAttentionOutput, feedForwardOutput))
	return output
}

type DecoderLayer struct {
	selfAttention    *MultiHeadAttention
	encoderAttention *MultiHeadAttention
	feedForward      *FeedForward
}

// Decode applies the Transformer decoder layer to decode input sequences
func (layer *DecoderLayer) Decode(input [][]float64, encoderOutput [][]float64) [][]float64 {
	// Apply Multi-Head Attention mechanism for self-attention
	selfAttentionOutput := layer.selfAttention.Decode(input, input)

	// Apply add-and-normalize layer
	selfAttentionOutput = NormalizeMat(ResidualConnectionMat(input, selfAttentionOutput))

	// Apply Multi-Head Attention mechanism for encoder-decoder attention
	encoderAttentionOutput := layer.encoderAttention.Decode(selfAttentionOutput, encoderOutput)

	// Apply add-and-normalize layer
	encoderAttentionOutput = NormalizeMat(ResidualConnectionMat(selfAttentionOutput, encoderAttentionOutput))

	// Apply FeedForward layer
	feedForwardOutput := layer.feedForward.Decode(encoderAttentionOutput)

	// Apply add-and-normalize layer
	output := NormalizeMat(ResidualConnectionMat(encoderAttentionOutput, feedForwardOutput))

	return output
}

type MultiHeadAttention struct {
	keys             [][]float64
	values           [][]float64
	queries          [][]float64
	contextVector    []float64
	outputProjection [][]float64 // Output projection parameters
	softmax          [][]float64 // Softmax scaling factors
	attentionScale   float64     // Attention scale factor
	heads            []*AttentionHead
}

type AttentionHead struct {
	wq, wk, wv [][]float64
}

func NewMultiHeadAttention(numHeads int, in, out int) *MultiHeadAttention {
	attention := &MultiHeadAttention{
		heads: make([]*AttentionHead, numHeads),
	}

	for i := 0; i < numHeads; i++ {
		attention.heads[i] = &AttentionHead{
			wq: xavierInitialization(in, out),
			wk: xavierInitialization(in, out),
			wv: xavierInitialization(in, out),
		}
	}

	// Initialize parameters for output projection
	outputProjection := xavierInitialization(numHeads, out)
	attention.outputProjection = outputProjection

	// Initialize softmax scaling factors
	sf := make([][]float64, in)
	for i := range sf {
		sf[i] = make([]float64, out)
		for j := range sf[i] {
			sf[i][j] = 1.0 / float64(out) // Initialize with uniform scaling
		}
	}
	attention.softmax = sf

	// Initialize attention scale factor
	attentionScale := math.Sqrt(float64(out))
	attention.attentionScale = attentionScale

	return attention
}

func (attention *MultiHeadAttention) Encode(input [][]float64) [][]float64 {
	// Compute queries, keys, and values for each head
	numHeads := len(attention.heads)
	queries := make([][][]float64, numHeads)
	keys := make([][][]float64, numHeads)
	values := make([][][]float64, numHeads)
	for i := 0; i < numHeads; i++ {
		queries[i] = matrixDot(input, attention.heads[i].wq)
		keys[i] = matrixDot(input, attention.heads[i].wk)
		values[i] = matrixDot(input, attention.heads[i].wv)
	}

	// Apply each head of attention mechanism
	attentionOutputs := make([][]float64, len(input))
	for i := range input {
		// Initialize output for the current input
		output := make([]float64, len(input[i]))

		// Apply attention for each head and concatenate the results
		for j := 0; j < numHeads; j++ {
			attentionHead := ApplyScaledDotProductAttention(queries[j], keys[j], values[j], attention.softmax, attention.attentionScale)
			for k := range attentionHead {
				output[k] += attentionHead[k] * attention.outputProjection[j][k]
			}
		}
		attentionOutputs[i] = output
	}

	return attentionOutputs
}

// ApplyScaledDotProductAttention computes the scaled dot-product attention given queries, keys, and values
func ApplyScaledDotProductAttention(queries [][]float64, keys [][]float64, values [][]float64, softmax [][]float64, scale float64) []float64 {
	// Compute attention scores
	attentionScores := make([][]float64, len(queries))
	for i := range queries {
		attentionScores[i] = make([]float64, len(keys[0]))
		for j := range keys {
			attentionScores[i][j] = dotProduct(queries[i], keys[j]) / scale
		}
	}

	// Apply softmax to attention scores
	for i := range attentionScores {
		attentionScores[i] = applySoftmax(attentionScores[i], softmax[i])
	}

	// Weighted sum of values using attention scores
	output := make([]float64, len(queries[0]))
	for i := range values {
		for j := range values[i] {
			output[j] += attentionScores[i][j] * values[i][j]
		}
	}

	return output
}

// dotProduct computes the dot product of two vectors
func dotProduct(vec1, vec2 []float64) float64 {
	result := 0.0
	for i := range vec1 {
		result += vec1[i] * vec2[i]
	}
	return result
}

func (attention *MultiHeadAttention) Decode(input [][]float64, encoderOutput [][]float64) [][]float64 {
	// Compute queries, keys, and values for each head using the input and encoder output
	numHeads := len(attention.heads)
	queries := make([][][]float64, numHeads)
	keys := make([][][]float64, numHeads)
	values := make([][][]float64, numHeads)
	for i := 0; i < numHeads; i++ {
		fmt.Printf("wq = %+v | %+v\n", len(attention.heads[i].wq), len(attention.heads[i].wq[0]))

		queries[i] = matrixDot(input, attention.heads[i].wq)
		keys[i] = matrixDot(encoderOutput, attention.heads[i].wk)
		values[i] = matrixDot(encoderOutput, attention.heads[i].wv)
	}

	// Apply each head of attention mechanism
	attentionOutputs := make([][]float64, len(input))
	for i := range input {
		// Initialize output for the current input
		output := make([]float64, len(input[i]))

		// Apply attention for each head and concatenate the results
		for j := 0; j < numHeads; j++ {
			attentionHead := ApplyScaledDotProductAttention(queries[j], keys[j], values[j], attention.softmax, attention.attentionScale)
			for k := range attentionHead {
				output[k] += attentionHead[k] * attention.outputProjection[j][k]
			}
		}
		attentionOutputs[i] = output
	}

	return attentionOutputs
}

// BackProp computes the gradients of the loss function and updates the parameters of the multi-head attention mechanism
func (attention *MultiHeadAttention) BackProp(loss [][]float64, learningRate float64) {
	// Compute gradients of the loss function with respect to the output projection matrix
	dLossOutput := loss // Assuming loss is directly propagated from the next layer
	dOutputProjection := matrixDotTranspose(dLossOutput, attention.values)

	// Compute gradients for the softmax layer
	dSoftmax := matrixDotTranspose(dLossOutput, attention.keys)

	numHeads := len(attention.heads)
	// Compute gradients for the query, key, and value projections for each head
	dQueryProjections := make([][][]float64, numHeads)
	dKeyProjections := make([][][]float64, numHeads)
	dValueProjections := make([][][]float64, numHeads)
	for i := 0; i < numHeads; i++ {
		dQueryProjections[i] = matrixDot(dSoftmax, attention.softmax)
		dKeyProjections[i] = matrixDot(dSoftmax, attention.softmax)
		dValueProjections[i] = matrixDot(dOutputProjection, attention.softmax)
	}

	// Update parameters of the attention mechanism using the gradients and the learning rate
	updateParameters(attention.outputProjection, dOutputProjection, learningRate)
	updateParameters(attention.softmax, dSoftmax, learningRate)
	for i := range attention.heads {
		updateParameters(attention.heads[i].wq, dQueryProjections[i], learningRate)
		updateParameters(attention.heads[i].wk, dKeyProjections[i], learningRate)
		updateParameters(attention.heads[i].wv, dValueProjections[i], learningRate)
	}
}

// updateParameters updates the parameters of a layer using the gradients and the learning rate
func updateParameters(parameters [][]float64, gradients [][]float64, learningRate float64) {
	for i := range parameters {
		for j := range parameters[i] {
			parameters[i][j] -= learningRate * gradients[i][j]
		}
	}
}

func backpropagateMultiHeadAttention(attention *MultiHeadAttention, loss float64, learningRate float64) {
	// Backpropagate through the second linear transformation
	dWv := make([][]float64, len(attention.values))
	for i := range attention.values {
		dWv[i] = make([]float64, len(attention.values[i]))
	}

	// Compute gradients for values matrix
	for i := range attention.values {
		for j := range attention.values[i] {
			dWv[i][j] = loss * attention.contextVector[j] * learningRate
		}
	}

	// Update weights for the values matrix
	for i := range attention.values {
		for j := range attention.values[i] {
			attention.values[i][j] -= dWv[i][j]
		}
	}

	// Compute gradients for keys and queries matrices
	dWk := make([][]float64, len(attention.keys))
	dWq := make([][]float64, len(attention.queries))
	for i := range attention.keys {
		dWk[i] = make([]float64, len(attention.keys[i]))
		dWq[i] = make([]float64, len(attention.queries[i]))
	}

	for i := range attention.keys {
		for j := range attention.keys[i] {
			dWk[i][j] = loss * attention.contextVector[j] * learningRate
			dWq[i][j] = loss * attention.contextVector[j] * learningRate
		}
	}

	// Update weights for the keys and queries matrices
	for i := range attention.keys {
		for j := range attention.keys[i] {
			attention.keys[i][j] -= dWk[i][j]
			attention.queries[i][j] -= dWq[i][j]
		}
	}
}

type meta struct {
	kind  string
	dim   []int
	layer []int
}

type FeedForward struct {
	meta   meta
	input  [][]float64
	output [][]float64
	w1, w2 [][]float64
}

func NewFeedForward(meta meta, in, out int) *FeedForward {
	return &FeedForward{
		meta: meta,
		w1:   xavierInitialization(in, in),
		w2:   xavierInitialization(out, in),
	}
}

// XavierInitialization initializes the weight matrices using Xavier initialization
func xavierInitialization(rows, cols int) [][]float64 {
	scale := math.Sqrt(2.0 / float64(rows*cols))
	matrix := make([][]float64, rows)
	for i := range matrix {
		matrix[i] = make([]float64, cols)
		for j := range matrix[i] {
			matrix[i][j] = rand.NormFloat64() * scale
		}
	}
	return matrix
}

// initializeWeights initializes the weights of a projection matrix with random values
func initializeWeights(inputSize, outputSize int) [][]float64 {
	weights := make([][]float64, inputSize)
	for i := range weights {
		weights[i] = make([]float64, outputSize)
		for j := range weights[i] {
			weights[i][j] = rand.NormFloat64() // Initialize with random values
		}
	}
	return weights
}

func (ffn *FeedForward) Encode(inputSequence [][]float64) [][]float64 {
	// Initialize an empty slice to store the output sequence
	outputSequence := make([][]float64, len(inputSequence))

	ffn.input = copyMat(inputSequence)
	// Iterate through each token in the input sequence
	for i := range inputSequence {
		// Perform the first linear transformation
		intermediate := linearTransform(inputSequence[i], ffn.w1)

		// Apply ReLU activation function
		for j := range intermediate {
			intermediate[j] = math.Max(0, intermediate[j])
		}

		// Perform the second linear transformation
		output := linearTransform(intermediate, ffn.w2)

		// Apply residual connection and layer normalization
		ResidualConnectionVec(inputSequence[i], output)
		outputSequence[i] = NormalizeVec(output)
	}
	ffn.output = copyMat(outputSequence)
	return outputSequence
}

func (ffn *FeedForward) Decode(inputSequence [][]float64) [][]float64 {
	// Initialize an empty slice to store the output sequence
	outputSequence := make([][]float64, len(inputSequence))
	ffn.input = copyMat(inputSequence)
	// Iterate through each token in the input sequence
	for i := range inputSequence {
		// Perform the first linear transformation
		intermediate := linearTransform(inputSequence[i], ffn.w1)

		// Apply ReLU activation function
		for j := range intermediate {
			intermediate[j] = math.Max(0, intermediate[j])
		}

		// Perform the second linear transformation
		output := linearTransform(intermediate, ffn.w2)

		// Apply residual connection and layer normalization
		ResidualConnectionVec(inputSequence[i], output)
		outputSequence[i] = NormalizeVec(output)
	}
	ffn.output = copyMat(outputSequence)
	return outputSequence
}

// BackProp computes the gradients of the loss function and updates the parameters of the feedforward layer
func (ffn *FeedForward) BackProp(loss [][]float64, learningRate float64) {
	// Initialize gradients for each parameter
	dw1 := make([][]float64, len(ffn.w1))
	dw2 := make([][]float64, len(ffn.w2))

	// Compute gradients of the loss function with respect to the parameters of the layer
	for i := range loss {
		// Compute gradient with respect to the output
		// Compute gradient with respect to the output of the feedforward layer
		dLossOutput := loss[i]

		// Backpropagate through the second linear transformation
		dOutputHidden := dotMat(transpose(ffn.w2), dLossOutput)
		dOutputHidden = applyReLUderivativeVec(dOutputHidden) // Applying derivative of ReLU
		dHiddenInput := dotMat(ffn.w2, dOutputHidden)         // Gradient of the loss w.r.t. the input of the second linear transformation

		// Compute gradients with respect to the parameters of the second linear transformation
		for j := range dw2 {
			for k := range dw2[j] {
				dw2[j][k] += dOutputHidden[j] * ffn.input[i][k]
			}
		}

		// Backpropagate through the first linear transformation
		dInputHidden := dotMat(transpose(ffn.w1), dHiddenInput)
		dInputHidden = applyReLUderivativeVec(dInputHidden) // Applying derivative of ReLU

		// Compute gradients with respect to the parameters of the first linear transformation
		for j := range dw1 {
			for k := range dw1[j] {
				dw1[j][k] += dInputHidden[j] * ffn.input[i][k]
			}
		}
	}

	// Update parameters of the feedforward layer using the gradients and the learning rate
	for i := range ffn.w1 {
		for j := range ffn.w1[i] {
			ffn.w1[i][j] -= learningRate * dw1[i][j] / float64(len(loss))
		}
	}
	for i := range ffn.w2 {
		for j := range ffn.w2[i] {
			ffn.w2[i][j] -= learningRate * dw2[i][j] / float64(len(loss))
		}
	}
}

// matrixDot computes the dot product of two matrices A and B
func matrixDot(A, B [][]float64) [][]float64 {
	m := len(A)
	n := len(B[0])
	p := len(B)

	result := make([][]float64, m)
	for i := range result {
		result[i] = make([]float64, n)
		for j := range result[i] {
			for k := 0; k < p; k++ {
				result[i][j] += A[i][k] * B[k][j]
			}
		}
	}
	return result
}

// matrixDotTranspose computes the dot product of a matrix A and the transpose of matrix B
func matrixDotTranspose(A, B [][]float64) [][]float64 {
	m := len(A)
	n := len(B[0])
	p := len(B)

	result := make([][]float64, m)
	for i := range result {
		result[i] = make([]float64, n)
		for j := range result[i] {
			for k := 0; k < p; k++ {
				result[i][j] += A[i][k] * B[k][j]
			}
		}
	}
	return result
}

// applyReLUderivativeMat applies the derivative of the ReLU activation function element-wise to the input matrix
func applyReLUderivativeMat(matrix [][]float64) [][]float64 {
	result := make([][]float64, len(matrix))
	for i := range matrix {
		result[i] = make([]float64, len(matrix[i]))
		for j := range matrix[i] {
			if matrix[i][j] > 0 {
				result[i][j] = 1.0
			} else {
				result[i][j] = 0.0
			}
		}
	}
	return result
}

// applyReLUderivativeVec applies the derivative of the ReLU activation function element-wise to the input slice
func applyReLUderivativeVec(input []float64) []float64 {
	result := make([]float64, len(input))
	for i, val := range input {
		if val > 0 {
			result[i] = 1.0
		} else {
			result[i] = 0.0
		}
	}
	return result
}
func backpropagateFeedForward(ffn *FeedForward, loss float64, learningRate float64) {
	// Backpropagate through the second linear transformation
	dW2 := make([][]float64, len(ffn.w2))
	for i := range ffn.w2 {
		dW2[i] = make([]float64, len(ffn.w2[i]))
	}

	fmt.Printf("ffn.input = %+v|%+v\n", len(ffn.input), len(ffn.input[0]))
	fmt.Printf("ffn.output = %+v|%+v\n", len(ffn.output), len(ffn.output[0]))
	fmt.Printf("ffn.w1 = %+v|%+v\n", len(ffn.w1), len(ffn.w1[0]))
	fmt.Printf("ffn.w2 = %+v|%+v\n", len(ffn.w2), len(ffn.w2[0]))

	fmt.Printf("ffn.meta = %+v\n", ffn.meta)
	for i := range ffn.w2 {
		for j := range ffn.w2[i] {
			dW2[i][j] = loss * ffn.input[j][i] * reluDerivative(ffn.output[j][i]) * learningRate
		}
	}

	// Update weights for the second linear transformation
	for i := range ffn.w2 {
		for j := range ffn.w2[i] {
			ffn.w2[i][j] -= dW2[i][j]
		}
	}

	// Compute loss for the first linear transformation
	dLoss := make([]float64, len(ffn.input))
	for i := range ffn.input {
		dLoss[i] = 0.0
		for j := range ffn.w2 {
			dLoss[i] += loss * ffn.w2[j][i] * reluDerivative(ffn.output[i][j]) * learningRate
		}
	}

	// Backpropagate through the first linear transformation
	dW1 := make([][]float64, len(ffn.w1))
	for i := range ffn.w1 {
		dW1[i] = make([]float64, len(ffn.w1[i]))
	}

	for i := range ffn.w1 {
		for j := range ffn.w1[i] {
			dW1[i][j] = dLoss[j] * ffn.input[i][j] * reluDerivative(ffn.input[i][j]) * learningRate
		}
	}

	// Update weights for the first linear transformation
	for i := range ffn.w1 {
		for j := range ffn.w1[i] {
			ffn.w1[i][j] -= dW1[i][j]
		}
	}
}

// ResidualConnectionMat adds a residual connection between two tensors
func ResidualConnectionMat(input, output [][]float64) [][]float64 {
	// Pad or truncate input tensor to match the size of the output tensor
	paddedInput := padOrTruncateMat(input, len(output))

	// Add the output of the layer to the input tensor
	combined := make([][]float64, len(output))
	for i := range output {
		combined[i] = make([]float64, len(output[i]))
		paddedInput[i] = padOrTruncateVec(paddedInput[i], len(output[i]))
		for j := range output[i] {
			combined[i][j] = output[i][j] + paddedInput[i][j]
		}
	}

	// Optionally apply normalization
	return NormalizeMat(combined)
}

// padOrTruncateMat pads or truncates a tensor to the specified size
func padOrTruncateMat(tensor [][]float64, size int) [][]float64 {
	result := make([][]float64, len(tensor))
	for i := range result {
		result[i] = make([]float64, size)
		for j := range result[i] {
			if j < len(tensor[i]) {
				result[i][j] = tensor[i][j]
			} else {
				result[i][j] = 0.0 // Pad with zeros
			}
		}
	}
	return result
}

// NormalizeMat normalizes the input tensor across the feature dimension
func NormalizeMat(tensor [][]float64) [][]float64 {
	batchMean := make([]float64, len(tensor[0]))
	batchVar := make([]float64, len(tensor[0]))

	// Compute mean and variance across feature dimension
	for _, example := range tensor {
		for j, value := range example {
			batchMean[j] += value
		}
	}
	for j := range batchMean {
		batchMean[j] /= float64(len(tensor))
	}

	for _, example := range tensor {
		for j, value := range example {
			batchVar[j] += math.Pow(value-batchMean[j], 2)
		}
	}
	for j := range batchVar {
		batchVar[j] /= float64(len(tensor))
	}

	// Normalize each example
	normalizedTensor := make([][]float64, len(tensor))
	for i, example := range tensor {
		normalizedExample := make([]float64, len(example))
		for j, value := range example {
			normalizedExample[j] = (value - batchMean[j]) / math.Sqrt(batchVar[j]+epsilon)
		}
		normalizedTensor[i] = normalizedExample
	}

	return normalizedTensor
}

// ResidualConnectionVec adds a residual connection between two tensors
func ResidualConnectionVec(input, output []float64) []float64 {
	// Pad or truncate input tensor to match the size of the output tensor
	paddedInput := padOrTruncateVec(input, len(output))

	// Add the output of the layer to the input tensor
	combined := make([]float64, len(output))
	for i := range output {
		combined[i] = output[i] + paddedInput[i]
	}

	// Optionally apply normalization
	return NormalizeVec(combined)
}

// padOrTruncateVec pads or truncates a tensor to the specified size
func padOrTruncateVec(tensor []float64, size int) []float64 {
	result := make([]float64, size)
	for i := range result {
		if i < len(tensor) {
			result[i] = tensor[i]
		} else {
			result[i] = 0.0 // Pad with zeros
		}
	}
	return result
}

// NormalizeVec normalizes the input tensor across the feature dimension
func NormalizeVec(tensor []float64) []float64 {
	batchMean := 0.0
	batchVar := 0.0

	// Compute mean and variance across feature dimension
	for _, value := range tensor {
		batchMean += value
	}
	batchMean /= float64(len(tensor))

	for _, value := range tensor {
		batchVar += math.Pow(value-batchMean, 2)
	}
	batchVar /= float64(len(tensor))

	// Normalize each example
	normalizedTensor := make([]float64, len(tensor))
	for i, value := range tensor {
		normalizedTensor[i] = (value - batchMean) / math.Sqrt(batchVar+epsilon)
	}

	return normalizedTensor
}

func linearTransform(input []float64, weights [][]float64) []float64 {
	// Check if dimensions are compatible
	if len(input) != len(weights[0]) {
		panic("Input dimension does not match weight matrix dimension")
	}

	// Initialize an empty slice to store the output vector
	output := make([]float64, len(weights))

	// Perform matrix multiplication and bias addition
	for i := range weights {
		for j := range input {
			output[i] += input[j] * weights[i][j]
		}
	}

	return output
}

func multiTransform(input []float64, heads []*AttentionHead, get func(head *AttentionHead) [][]float64) []float64 {
	output := make([]float64, len(input))
	for _, head := range heads {
		w := get(head)
		for j := range input {
			for k := range w[j] {
				output[k] += input[j] * w[j][k]
			}
		}
	}
	return output
}

// applySoftmax applies the softmax function to a vector
func applySoftmax(vector []float64, softmax []float64) []float64 {
	result := make([]float64, len(vector))
	expSum := 0.0
	for i := range vector {
		expSum += math.Exp(vector[i] * softmax[i])
	}
	for i := range vector {
		result[i] = math.Exp(vector[i]*softmax[i]) / expSum
	}
	return result
}

func softmax(scores []float64) []float64 {
	// Compute the maximum score
	maxScore := math.Inf(-1)
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}

	// Compute the exponentials of the scores
	expScores := make([]float64, len(scores))
	sumExp := 0.0
	for i, score := range scores {
		expScores[i] = math.Exp(score - maxScore)
		sumExp += expScores[i]
	}

	// Normalize the scores to obtain the attention weights
	attentionWeights := make([]float64, len(scores))
	for i, expScore := range expScores {
		attentionWeights[i] = expScore / sumExp
	}

	return attentionWeights
}

func softmaxWithMasking(scores []float64, currentIndex int) []float64 {
	// Compute the maximum score
	maxScore := math.Inf(-1)
	for _, score := range scores {
		if score > maxScore && score != math.Inf(1) {
			maxScore = score
		}
	}

	// Compute the exponentials of the scores
	expScores := make([]float64, len(scores))
	sumExp := 0.0
	for i, score := range scores {
		if i > currentIndex {
			expScores[i] = 0 // Apply masking
		} else {
			expScores[i] = math.Exp(score - maxScore)
			sumExp += expScores[i]
		}
	}
	// Normalize the scores to obtain the attention weights
	attentionWeights := make([]float64, len(scores))
	for i, expScore := range expScores {
		attentionWeights[i] = expScore / sumExp
	}

	return attentionWeights
}

// Example function to compute loss (e.g., mean squared error)
func computeLoss(output, target [][]float64) float64 {
	var loss float64
	for i := range output {
		for j := range output[i] {
			loss += (output[i][j] - target[i][j]) * (output[i][j] - target[i][j])
		}
	}
	return loss / float64(len(output))
}

// CalculateLossMatrix calculates the loss as a matrix given the predicted output and the target output
func CalculateLossMatrix(predictedOutput [][]float64, targetOutput [][]float64) [][]float64 {
	// Ensure that the dimensions of predictedOutput and targetOutput are the same
	if len(predictedOutput) != len(targetOutput) || len(predictedOutput[0]) != len(targetOutput[0]) {
		panic("Dimensions of predicted output and target output must be the same")
	}

	// Initialize the loss matrix
	lossMatrix := make([][]float64, len(predictedOutput))
	for i := range lossMatrix {
		lossMatrix[i] = make([]float64, len(predictedOutput[0]))
	}

	// Calculate the loss for each element of the matrices
	for i := range predictedOutput {
		for j := range predictedOutput[i] {
			lossMatrix[i][j] = predictedOutput[i][j] - targetOutput[i][j]
		}
	}

	return lossMatrix
}

func reluDerivative(x float64) float64 {
	if x > 0 {
		return 1
	} else {
		return 0
	}
}
