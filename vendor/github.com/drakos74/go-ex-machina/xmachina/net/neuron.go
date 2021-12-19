package net

import (
	"fmt"

	"github.com/drakos74/go-ex-machina/xmath"

	"github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/rs/zerolog/log"
)

// Weights encapsulates all needed parameters to apply to the neuron attributes
type Weights struct {
	W xmath.Matrix
	B xmath.Vector
}

// NewWeights creates a new set of weights and bias.
func NewWeights(n, m int, weightsGenerator, biasGenerator xmath.VectorGenerator) *Weights {
	return &Weights{
		W: xmath.Mat(m).Generate(n, weightsGenerator),
		B: xmath.Vec(m).Generate(biasGenerator),
	}
}

// Neuron is a minimal computation unit with an activation function.
// It is effectively a collection of perceptrons so not the smallest unit after all,
// but it allows for extension in more general cases than feed forward neural nets.
type Neuron interface {
	Op
	Meta() Meta
	Weights() *Weights
}

// Meta defines the metadata for the neuron.
type Meta struct {
	Layer int
	Index int
	ID    string
}

func (m Meta) WithID(id string) Meta {
	return Meta{
		Layer: m.Layer,
		Index: m.Index,
		ID:    id,
	}
}

// ActivationCell is the basic implementation for the neuron.
type ActivationCell struct {
	learning      ml.Module
	weights       *Weights
	meta          Meta
	input, output xmath.Vector
}

// NewActivationCell creates a new ml neuron.
func NewActivationCell(n, m int, module ml.Module, weights *Weights, meta Meta) Neuron {
	return &ActivationCell{
		learning: module,
		weights:  weights,
		meta:     meta,
		input:    xmath.Vec(n),
		output:   xmath.Vec(m),
	}
}

// Fwd applies the forward pass logic for the neuron.
func (n *ActivationCell) Fwd(v xmath.Vector) xmath.Vector {
	xmath.MustHaveSameSize(v, n.input)
	// keep a copy of the input in memory
	n.input = v
	// combine with the weights
	w := n.weights.W.Prod(v)
	// add bias
	z := w.Add(n.weights.B)
	// apply activation
	n.output = z.Op(n.learning.F)
	return n.output
}

// Bwd applies the backward propagation logic for the neuron,
// while it also updates the weights and biases accordingly.
func (n *ActivationCell) Bwd(diff xmath.Vector) xmath.Vector {
	log.Trace().
		Str("meta", fmt.Sprintf("%+v", n.meta)).
		Floats64("diff", diff).
		Msg("train-diff")
	// find the derivative of the output
	deriv := n.output.Op(n.learning.D)
	log.Trace().
		Str("meta", fmt.Sprintf("%+v", n.meta)).
		Floats64("deriv", deriv).
		Msg("de-activation")
	// find the gradient compared to the diff
	grad := deriv.X(diff)
	log.Trace().
		Str("meta", fmt.Sprintf("%+v", n.meta)).
		Floats64("grad", grad).
		Msg("gradient-descent")
	// compute loss for previous layer
	loss := n.weights.W.T().Prod(grad)
	log.Trace().
		Str("meta", fmt.Sprintf("%+v", n.meta)).
		Floats64("loss", loss).
		Msg("loss")
	// update weights and bias
	dW := grad.Prod(n.input)
	n.weights.W = n.weights.W.Add(dW.Mult(n.learning.WRate()))
	n.weights.B = n.weights.B.Add(grad.Mult(n.learning.BRate()))
	// return the loss to the previous layer
	return loss
}

// Meta returns the metadata for the neuron.
func (n ActivationCell) Meta() Meta {
	return n.meta
}

// Weights returns all the required constants needed to re-build the neuron state.
func (n ActivationCell) Weights() *Weights {
	return n.weights
}

// NoOpCell is a neuron cell that does not do anything.
type NoOpCell struct {
	meta Meta
}

// TODO : move to an Op , as it has no weights
// NoOp creates a new no-op cell.
func NoOp(n, m int, meta Meta) Neuron {
	if n != m {
		panic(fmt.Sprintf("cannot make a no-op cell with different input and output sizes %v vs %v", n, m))
	}
	return &NoOpCell{meta: meta}
}

// Fwd propagates the input as output.
func (n *NoOpCell) Fwd(x xmath.Vector) xmath.Vector {
	return x
}

// Bwd propagates the input as output.
func (n *NoOpCell) Bwd(dy xmath.Vector) xmath.Vector {
	return dy
}

// Meta returns the metadata for the cell.
func (n *NoOpCell) Meta() Meta {
	return n.meta
}

// Weights returns no weights, as this is a no-op cell.
func (n *NoOpCell) Weights() *Weights {
	return nil
}

// TODO : move to an Op , as it has no weights
// SoftCell is the basic implementation for the neuron.
type SoftCell struct {
	softmax       ml.SoftActivation
	meta          Meta
	input, output xmath.Vector
}

// NewSoftCell creates a new ml neuron.
func NewSoftCell(n, m int, meta Meta) Neuron {
	// override the ml module with softmax
	return &SoftCell{
		softmax: ml.SoftMax{},
		meta:    meta,
		input:   xmath.Vec(n),
		output:  xmath.Vec(m),
	}
}

// Fwd applies the forward pass logic for the neuron.
func (n *SoftCell) Fwd(v xmath.Vector) xmath.Vector {
	xmath.MustHaveSameSize(v, n.input)
	// keep a copy of the input in memory
	n.input = v
	n.output = n.softmax.F(v)
	return n.output
}

// Bwd applies the backward propagation logic for the neuron,
// while it also updates the weights and biases accordingly.
func (n *SoftCell) Bwd(diff xmath.Vector) xmath.Vector {
	return n.softmax.D(n.output).Prod(diff)
}

// Meta returns the metadata for the neuron.
func (n SoftCell) Meta() Meta {
	return n.meta
}

// Weights returns all the required constants needed to re-build the neuron state.
func (n SoftCell) Weights() *Weights {
	return nil
}

// WeightCell is the basic implementation for the matrix multiplication neuron.
// because there is no activation the output and weights are prone to explode or behave badly.
// This typeof neuron is to be used in combination with others within a compound neuron i.e. rnn
type WeightCell struct {
	learning      ml.Learning
	weights       *Weights
	meta          Meta
	input, output xmath.Vector
}

// NewWeightCell creates a new ml neuron.
func NewWeightCell(n, m int, learning ml.Module, weights *Weights, meta Meta) Neuron {
	log.Trace().
		Str("meta", fmt.Sprintf("%+v", meta)).
		Int("input", n).
		Int("output", m).
		Msg("create")
	return &WeightCell{
		learning: *learning.Learning,
		weights:  weights,
		meta:     meta,
		input:    xmath.Vec(n),
		output:   xmath.Vec(m),
	}
}

// Fwd applies the forward pass logic for the neuron.
func (w *WeightCell) Fwd(v xmath.Vector) xmath.Vector {
	xmath.MustHaveSameSize(v, w.input)
	// keep a copy of the input in memory
	w.input = v
	// combine with the weights
	m := w.weights.W.Prod(v)
	w.output = m.Add(w.weights.B)
	return m
}

// Bwd applies the backward propagation logic for the neuron,
// while it also updates the weights and biases accordingly.
func (w *WeightCell) Bwd(diff xmath.Vector) xmath.Vector {
	log.Trace().
		Str("meta", fmt.Sprintf("%+v", w.meta)).
		Floats64("diff", diff).
		Msg("train-diff")
	// compute loss for previous layer
	dw := w.weights.W.T().Prod(diff)
	log.Trace().
		Str("meta", fmt.Sprintf("%+v", w.meta)).
		Floats64("loss", dw).
		Msg("loss")
	// update weights and bias
	dW := diff.Prod(w.input)
	w.weights.W = w.weights.W.Add(dW.Mult(w.learning.WRate()))
	w.weights.B = w.weights.B.Add(diff.Mult(w.learning.BRate()))

	// return the loss to the previous layer
	return dw
}

// Meta returns the metadata for the neuron.
func (w WeightCell) Meta() Meta {
	return w.meta
}

// Weights returns all the required constants needed to re-build the neuron state.
func (w WeightCell) Weights() *Weights {
	return w.weights
}

// NeuronFactory is a factory for construction of neuron within the context of a neuron layer / network
type NeuronFactory func(n, m int, meta Meta) Neuron

// NeuronBuilder is a helper struct to create a neuron factory
type NeuronBuilder struct {
	module           *ml.Module
	weightsGenerator xmath.VectorGenerator
	biasGenerator    xmath.VectorGenerator
}

// NewBuilder creates a default neuron factory builder with everything set,
// which can be used directly, or adjusted with the builder methods.
func NewBuilder() *NeuronBuilder {
	return &NeuronBuilder{
		module:           ml.Base(),
		weightsGenerator: xmath.Const(0.5),
		biasGenerator:    xmath.Const(0.5),
	}
}

// WithWeights specifies the starting weights for the neuron.
func (nb *NeuronBuilder) WithWeights(weightsGenerator xmath.VectorGenerator, biasGenerator xmath.VectorGenerator) *NeuronBuilder {
	nb.weightsGenerator = weightsGenerator
	nb.biasGenerator = biasGenerator
	return nb
}

// WithModule specifies the ml module to use.
func (nb *NeuronBuilder) WithModule(module *ml.Module) *NeuronBuilder {
	nb.module = module
	return nb
}

// NeuronConstructor defines a neuron constructor to be used with the neuron builder.
type NeuronConstructor func(n, m int, module ml.Module, weights *Weights, meta Meta) Neuron

// Factory returns a neuron factory.
func (nb *NeuronBuilder) Factory(constr NeuronConstructor) NeuronFactory {
	return func(n, m int, meta Meta) Neuron {
		log.Trace().Int("n-input", n).Int("m-output", m).Msg("create neuron")
		return constr(
			n, m,
			*nb.module,
			NewWeights(n, m, nb.weightsGenerator, nb.biasGenerator),
			meta,
		)
	}
}

// CellConstructor defines a cell constructor to be used with the neuron builder.
type CellConstructor func(n, m int, meta Meta) Neuron

// Factory returns a neuron factory.
func (nb *NeuronBuilder) CellFactory(constr CellConstructor) NeuronFactory {
	return func(n, m int, meta Meta) Neuron {
		log.Trace().Int("n-input", n).Int("m-output", m).Msg("create cell")
		return constr(
			n, m,
			meta,
		)
	}
}
