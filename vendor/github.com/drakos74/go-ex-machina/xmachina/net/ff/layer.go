package ff

import (
	"github.com/drakos74/go-ex-machina/xmachina/net"
	"github.com/drakos74/go-ex-machina/xmath"
)

// Layer represents a layer in the network,
// it will receive a vector of inputs and transform them into another vectopr of inputs.
// Not necessarily of the same size.
type Layer struct {
	n, m   int
	neuron net.Neuron
}

// NewLayer creates a new layer.
func NewLayer(n, m int, factory net.NeuronFactory, index int) *Layer {
	return &Layer{n: n, m: m, neuron: factory(n, m, net.Meta{
		Layer: index,
		Index: 0,
	})}
}

// Size returns the input and output size of the layer.
func (l *Layer) Size() (int, int) {
	return l.n, l.m
}

// Forward takes as input the outputs of all the neurons of the previous layer
// it returns the output of all the neurons of the current layer
func (l *Layer) Forward(v xmath.Vector) xmath.Vector {
	return l.neuron.Fwd(v)
}

// Backward receives all the errors/diffs from the following layer
// it returns the errors/diffs for the previous layer
func (l *Layer) Backward(err xmath.Vector) xmath.Vector {
	return l.neuron.Bwd(err)
}

// Weights returns the weights of the current layer for storing the network state.
func (l *Layer) Weights() map[net.Meta]net.Weights {
	return map[net.Meta]net.Weights{
		l.neuron.Meta(): *l.neuron.Weights(),
	}
}

type xVector struct {
	value xmath.Vector
	index int
}

type xFloat struct {
	value float64
	index int
}

type xLayer struct {
	pSize   int
	neurons []*xNeuron
	out     chan xFloat
	backOut chan xVector
}

func newXLayer(p, n int, factory NeuronFactory, index int) *xLayer {
	neurons := make([]*xNeuron, n)

	out := make(chan xFloat, n)
	backout := make(chan xVector, n)

	for i := 0; i < n; i++ {
		n := &xNeuron{
			Neuron: factory(p, net.Meta{
				Index: i,
				Layer: index,
			}),
			input:   make(chan xmath.Vector, 1),
			output:  out,
			backIn:  make(chan float64, 1),
			backOut: backout,
		}
		n.init()
		neurons[i] = n
	}
	return &xLayer{
		pSize:   p,
		neurons: neurons,
		out:     out,
		backOut: backout,
	}
}

func (xl *xLayer) Size() (int, int) {
	return len(xl.neurons), xl.pSize
}

func (xl *xLayer) Forward(v xmath.Vector) xmath.Vector {

	out := xmath.Vec(len(xl.neurons))

	for _, n := range xl.neurons {
		n.input <- v
	}

	c := 0
	for o := range xl.out {
		out[o.index] = o.value
		c++
		if c == len(xl.neurons) {
			break
		}
	}
	return out
}

func (xl *xLayer) Backward(err xmath.Vector) xmath.Vector {
	// we are building the error output for the previous layer
	dn := xmath.Mat(len(xl.neurons))

	for i, n := range xl.neurons {
		// and produce partial error for previous layer
		n.backIn <- err[i]
	}

	c := 0
	for o := range xl.backOut {
		dn[o.index] = o.value
		c++
		if c == len(xl.neurons) {
			break
		}
	}

	return dn.T().Sum()
}

func (xl *xLayer) Weights() map[net.Meta]net.Weights {
	sm, sb := xl.Size()
	m := xmath.Mat(sm)
	n := xmath.Vec(sb)
	for j := 0; j < len(xl.neurons); j++ {
		m[j] = xl.neurons[j].weights
		n[j] = xl.neurons[j].bias
	}
	return map[net.Meta]net.Weights{
		net.Meta{}: {
			W: m,
			B: n,
		},
	}
}
