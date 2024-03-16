package net

import (
	"fmt"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/go-ex-machina/xmath"
)

func NewMultiNetwork(nn ...Network) *MultiNetwork {
	return &MultiNetwork{
		net: nn,
	}
}

type MultiNetwork struct {
	net      []Network
	metadata ml.Metadata
}

func (m *MultiNetwork) Train(x [][]float64, y [][]float64) (ml.Metadata, error) {
	for i, n := range m.net {
		_, err := n.Train(x, y)
		if err != nil {
			return m.metadata, fmt.Errorf("error for multi-network at %d : %+v", i, err)

		}
	}
	return m.metadata, nil
}

func (m *MultiNetwork) Predict(x [][]float64) ([][]float64, ml.Metadata, error) {
	var prediction []float64
	for i, n := range m.net {
		pp, _, err := n.Predict(x)
		if err != nil {
			return [][]float64{prediction}, m.metadata, fmt.Errorf("error for multi-network at %d : %+v", i, err)
		}
		thisPrediction := last(pp)
		if prediction == nil {
			prediction = thisPrediction
		}
		for j, _ := range thisPrediction {
			if prediction[j] != thisPrediction[j] {
				return [][]float64{{0}}, m.metadata, nil
			}
		}
	}
	return [][]float64{prediction}, m.metadata, nil
}

func (m *MultiNetwork) Loss(actual, predicted [][]float64) []float64 {
	var loss xmath.Vector
	for _, n := range m.net {
		l := n.Loss(actual, predicted)
		if loss == nil {
			loss = l
		} else {
			loss = loss.Add(V(l))
		}
	}
	return loss
}

func (m *MultiNetwork) Config() mlmodel.Model {
	return mlmodel.Model{}
}

func (m *MultiNetwork) Load(key model.Key, detail mlmodel.Detail) error {
	return nil
}

func (m *MultiNetwork) Save(key model.Key, detail mlmodel.Detail) error {
	return nil
}
