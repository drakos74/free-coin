package net

import (
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/math/ml"
	xml "github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/drakos74/go-ex-machina/xmachina/net"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"

	"math"

	"github.com/drakos74/go-ex-machina/xmath"
)

const NN_KEY string = "net.NeuralNet"

type NeuralNet struct {
	cfg      mlmodel.Model
	metadata ml.Metadata
	net      *ff.Network
}

func NewNeuralNet(cfg mlmodel.Model) *NeuralNet {
	rate := xml.Learn(1, 0.1)

	initW := xmath.Rand(-1, 1, math.Sqrt)
	initB := xmath.Rand(-1, 1, math.Sqrt)
	network := ff.New(7, 3).
		Add(42, net.NewBuilder().
			WithModule(xml.Base().
				WithRate(rate).
				WithActivation(xml.TanH)).
			WithWeights(initW, initB).
			Factory(net.NewActivationCell)).
		Add(9, net.NewBuilder().
			WithModule(xml.Base().
				WithRate(rate).
				WithActivation(xml.TanH)).
			WithWeights(initW, initB).
			Factory(net.NewActivationCell)).
		Add(3, net.NewBuilder().
			WithModule(xml.Base().
				WithRate(rate).
				WithActivation(xml.TanH)).
			WithWeights(initW, initB).
			Factory(net.NewActivationCell)).
		Add(3, net.NewBuilder().CellFactory(net.NewSoftCell))
	network.Loss(xml.Pow)

	return &NeuralNet{
		cfg:      cfg,
		metadata: ml.NewMetadata(),
		net:      nil,
	}
}

func (n *NeuralNet) Train(x [][]float64, y [][]float64) (ml.Metadata, error) {
	lastX := x[len(x)-1]
	lastY := y[len(y)-1]
	inp := xmath.Vec(len(lastX)).With(lastX...)
	loss, _ := n.net.Train(inp, xmath.Vec(len(lastY)).With(lastY...))
	n.metadata.Loss = loss
	return n.metadata, nil
}

func (n *NeuralNet) Predict(x [][]float64) ([][]float64, ml.Metadata, error) {
	lastX := x[len(x)-1]
	inp := xmath.Vec(len(lastX)).With(lastX...)
	outp := n.net.Predict(inp)
	return [][]float64{outp}, n.metadata, nil
}

func (n *NeuralNet) Loss(actual, predicted [][]float64) []float64 {
	lastActual := actual[len(actual)-1]
	lastPredicted := predicted[len(predicted)-1]
	return xmath.Vec(len(lastActual)).With(lastActual...).
		Diff(xmath.Vec(len(lastPredicted)).With(lastPredicted...))
}

func (n *NeuralNet) Config() mlmodel.Model {
	return n.cfg

}
