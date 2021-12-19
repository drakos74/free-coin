package ff

import (
	"github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/drakos74/go-ex-machina/xmachina/net"
	"github.com/drakos74/go-ex-machina/xmath"
)

type memory struct {
	input  xmath.Vector
	output float64 //nolint
}

type learn struct {
	weights xmath.Vector
	bias    float64 //nolint
}

type Neuron struct {
	ml.Module
	net.Meta
	memory
	learn
}

func (n *Neuron) forward(v xmath.Vector) float64 {
	xmath.MustHaveSameSize(v, n.input)
	n.input = v
	n.output = n.Module.F(v.Dot(n.weights) + n.bias)
	return n.output
}

func (n *Neuron) backward(err float64) xmath.Vector {
	loss := xmath.Vec(len(n.weights))
	grad := n.Module.Grad(err, n.Module.D(n.output))
	for i, inp := range n.input {
		// create the error for the previous layer
		loss[i] = grad * n.weights[i]
		// we are updating the weights while going back as well
		n.weights[i] = n.weights[i] + n.Module.WRate()*grad*inp
		n.bias += grad * n.Module.BRate()
	}
	return loss
}

type xNeuron struct {
	*Neuron
	input   chan xmath.Vector
	output  chan xFloat
	backIn  chan float64
	backOut chan xVector
}

// init initialises the neurons to listen for incoming forward and backward directed data from the parent layer
func (xn *xNeuron) init() *xNeuron {
	go func(xn *xNeuron) {
		for {
			select {
			case v := <-xn.input:
				xn.output <- xFloat{
					value: xn.Neuron.forward(v),
					index: xn.Neuron.Meta.Index,
				}
			case e := <-xn.backIn:
				xn.backOut <- xVector{
					value: xn.Neuron.backward(e),
					index: xn.Neuron.Meta.Index,
				}
			}
		}
	}(xn)
	return xn
}

// TODO : the below are helper functions for the xNeuron and xNetwork implementation
// at some point they should be unified with the rest.

// NeuronFactory is a factory for construction of neuron within the context of a neuron layer / network
type NeuronFactory func(p int, meta net.Meta) *Neuron

var Perceptron = func(module *ml.Module, weights xmath.VectorGenerator) NeuronFactory {
	return func(p int, meta net.Meta) *Neuron {
		return &Neuron{
			Module: *module,
			memory: memory{
				input: xmath.Vec(p),
			},
			Meta: meta,
			learn: learn{
				weights: weights(p, meta.Index),
			},
		}
	}
}
