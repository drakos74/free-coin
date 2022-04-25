package net

import (
	"math"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"

	"github.com/drakos74/free-coin/internal/model"
)

type PolynomialRegression struct {
	SingleNetwork
	cfg mlmodel.Model
}

func ConstructPolynomialNetwork(threshold float64) func(cfg mlmodel.Model) Network {
	return func(cfg mlmodel.Model) Network {
		return NewPolynomialRegressionNetwork(cfg)
	}
}

func NewPolynomialRegressionNetwork(cfg mlmodel.Model) *PolynomialRegression {
	return &PolynomialRegression{
		SingleNetwork: NewSingleNetwork(),
		cfg:           cfg,
	}
}

func (p *PolynomialRegression) Model() mlmodel.Model {
	return p.cfg
}

func (p *PolynomialRegression) Train(ds *Dataset) (ModelResult, map[string]ModelResult) {
	if len(ds.Vectors) > 0 {
		v := ds.Vectors[len(ds.Vectors)-1]
		in := v.NewIn
		if in[2] > p.cfg.PrecisionThreshold {
			return ModelResult{
				Type:     model.Buy,
				Accuracy: in[2],
				OK:       true,
			}, make(map[string]ModelResult)
		} else if in[2] < -1*p.cfg.PrecisionThreshold {
			return ModelResult{
				Type:     model.Sell,
				Accuracy: math.Abs(in[2]),
				OK:       true,
			}, make(map[string]ModelResult)
		}
	}
	return ModelResult{}, make(map[string]ModelResult)
}
