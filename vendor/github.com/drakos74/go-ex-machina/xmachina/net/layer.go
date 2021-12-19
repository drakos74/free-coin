package net

import (
	"github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/drakos74/go-ex-machina/xmath"
)

// TODO : unify layer abstraction

// LayerBuilder allows to construct a layer implementation.
type LayerBuilder struct {
	n, m    int
	factory NeuronFactory
	loss    ml.Loss
}

// NewLayer creates a new layer builder.
func NewLayer() *LayerBuilder {
	return &LayerBuilder{}
}

// WithSize defines the input and output size for the layer.
// n is the inout vector size
// m is the output vector size
func (lb *LayerBuilder) WithSize(n, m int) *LayerBuilder {
	lb.n = n
	lb.m = m
	return lb
}

// WithNeuronFactory defines the neuron factory for the neurons of the layer.
func (lb *LayerBuilder) WithNeuronFactory(factory NeuronFactory) *LayerBuilder {
	lb.factory = factory
	return lb
}

// WithLoss defines the loss function for the layer.
func (lb *LayerBuilder) WithLoss(loss ml.Loss) *LayerBuilder {
	lb.loss = loss
	return lb
}

// NewNeuron generates a new neuron
func (lb LayerBuilder) NewNeuron(meta Meta) Neuron {
	return lb.factory(lb.n, lb.m, meta)
}

// Layer is the generic layer interface for an ml network.
type Layer interface {
	// F will take the input from the previous layer and generate an input for the next layer
	Forward(v xmath.Vector) xmath.Vector
	// Backward will take the loss from next layer and generate a loss for the previous layer
	Backward(dv xmath.Vector) xmath.Vector
	// Weights returns the current weight matrix for the layer
	Weights() map[Meta]Weights
	// Size returns the Size of the layer e.g. number of neurons
	Size() (int, int)
}

// RecurrentLayer is the generic layer interface for an ml recurrent network.
type RecurrentLayer interface {
	// F will take the input from the previous layer and generate an input for the next layer
	Forward(v xmath.Matrix) xmath.Matrix
	// Backward will take the loss from next layer and generate a loss for the previous layer
	Backward(dv xmath.Matrix) xmath.Matrix
	// Weights returns the current weight matrix for the layer
	Weights() map[Meta]Weights
	// Size returns the Size of the layer e.g. number of neurons
	Size() (n, x, h int)
}

// Clip defines the clipping range for weights
type Clip struct {
	W, B             float64
	wClipOp, bClipOp xmath.Op
}

// NewClip creates a new clip operation struct.
func NewClip(w, b float64) Clip {
	return Clip{
		W:       w,
		B:       b,
		wClipOp: xmath.Clip(-1*w, 1*w),
		bClipOp: xmath.Clip(-1*b, 1*b),
	}
}

// Apply applies the clipping to the weights.
func (c Clip) Apply(weights *Weights) {
	weights.W = weights.W.Op(c.wClipOp)
	weights.B = weights.B.Op(c.bClipOp)
}
