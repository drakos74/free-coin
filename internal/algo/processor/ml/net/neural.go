package net

import (
	"math"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/drakos74/go-ex-machina/xmachina/net"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/drakos74/go-ex-machina/xmath"
	"github.com/rs/zerolog/log"
)

// NNetwork defines an ml Network.
type NNetwork struct {
	SingleNetwork
	net *ff.Network
	cfg mlmodel.Model
}

func ConstructNeuralNetwork(network *ff.Network) func(cfg mlmodel.Model) Network {
	return func(cfg mlmodel.Model) Network {
		return NewNN(network, cfg)
	}
}

// NewNN creates a new neural Network.
func NewNN(network *ff.Network, cfg mlmodel.Model) *NNetwork {
	// tanh with softmax
	if network == nil {
		rate := ml.Learn(1, 0.1)

		initW := xmath.Rand(-1, 1, math.Sqrt)
		initB := xmath.Rand(-1, 1, math.Sqrt)
		network = ff.New(7, 3).
			Add(42, net.NewBuilder().
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

	return &NNetwork{net: network, cfg: cfg}
}

func (n *NNetwork) Model() mlmodel.Model {
	return n.cfg
}

func (n *NNetwork) Train(ds *Dataset) (ModelResult, map[string]ModelResult) {
	accuracy := math.MaxFloat64
	i := 0
	//for {
	acc, err := n.Fit(ds)
	if err != nil {
		log.Error().Err(err).Msg("error during training")
		return ModelResult{}, make(map[string]ModelResult)
	}
	//if Accuracy < 1 || i > 10 {
	accuracy = acc
	//break
	//}
	i++
	//}

	//if i < 10 && accuracy < 1 {
	t := n.Predict(ds)

	return ModelResult{
		Type:     t,
		Accuracy: accuracy,
		OK:       t != model.NoType,
	}, make(map[string]ModelResult)
	//}
	//
	//return model.NoType, 0.0, false

}

func (n *NNetwork) Fit(ds *Dataset) (float64, error) {
	l := 0.0
	for i := 0; i < len(ds.Vectors)-1; i++ {
		vv := ds.Vectors[i]
		inp := xmath.Vec(len(vv.PrevIn)).With(vv.PrevIn...)
		loss, _ := n.net.Train(inp, xmath.Vec(len(vv.PrevOut)).With(vv.PrevOut...))
		l += loss.Norm()
	}
	return l, nil
}

func (n *NNetwork) Predict(ds *Dataset) model.Type {

	last := ds.Vectors[len(ds.Vectors)-1]

	inp := xmath.Vec(len(last.NewIn)).With(last.NewIn...)

	outp := n.net.Predict(inp)

	if outp[0]-outp[2] > 0.0 {
		return model.Buy
	} else if outp[2]-outp[0] > 0.0 {
		return model.Sell
	}

	return model.NoType

}
