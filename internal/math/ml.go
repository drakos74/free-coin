package math

import (
	"math"

	"github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/drakos74/go-ex-machina/xmachina/net"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/drakos74/go-ex-machina/xmath"
)

// Network defines an ml network.
type Network struct {
	net *ff.Network
}

// NewML creates a new ml network.
func NewML(network *ff.Network) *Network {
	// tanh with softmax
	if network == nil {
		rate := ml.Learn(1, 0.1)

		initW := xmath.Rand(0, 1, math.Sqrt)
		initB := xmath.Rand(0, 1, math.Sqrt)
		network = ff.New(9, 3).
			Add(90, net.NewBuilder().
				WithModule(ml.Base().
					WithRate(rate).
					WithActivation(ml.TanH)).
				WithWeights(initW, initB).
				Factory(net.NewActivationCell)).
			Add(9, net.NewBuilder().
				WithModule(ml.Base().
					WithRate(rate).
					WithActivation(ml.TanH)).
				WithWeights(initW, initB).
				Factory(net.NewActivationCell)).
			Add(3, net.NewBuilder().
				WithModule(ml.Base().
					WithRate(rate).
					WithActivation(ml.TanH)).
				WithWeights(initW, initB).
				Factory(net.NewActivationCell)).
			Add(3, net.NewBuilder().CellFactory(net.NewSoftCell))
		network.Loss(ml.Pow)
	}

	return &Network{net: network}
}

// Train digests the input and returns the currently predicted output.
func (n *Network) Train(in, out []float64) float64 {

	inp := xmath.Vec(len(in)).With(in...)

	loss, _ := n.net.Train(inp, xmath.Vec(len(out)).With(out...))

	return loss.Norm()
}

// Predict returns the predicted output.
func (n *Network) Predict(in []float64) []float64 {

	inp := xmath.Vec(len(in)).With(in...)

	outp := n.net.Predict(inp)

	return outp
}
