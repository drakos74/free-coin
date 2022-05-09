package net

import (
	"math"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"

	"github.com/drakos74/free-coin/internal/model"
)

type PolynomialRegression struct {
	SingleNetwork
}

func ConstructPolynomialNetwork(threshold float64) func(cfg mlmodel.Model) Network {
	return func(cfg mlmodel.Model) Network {
		return NewPolynomialRegressionNetwork(cfg)
	}
}

func NewPolynomialRegressionNetwork(cfg mlmodel.Model) *PolynomialRegression {
	return &PolynomialRegression{
		SingleNetwork: NewSingleNetwork(cfg),
	}
}

func (p *PolynomialRegression) Train(ds *Dataset) ModelResult {
	config := p.SingleNetwork.config
	if len(ds.Vectors) > 0 {
		v := ds.Vectors[len(ds.Vectors)-1]
		in := v.NewIn
		if in[2] > config.PrecisionThreshold {
			return ModelResult{
				Type:     model.Buy,
				Accuracy: in[2],
				OK:       true,
			}
		} else if in[2] < -1*config.PrecisionThreshold {
			return ModelResult{
				Type:     model.Sell,
				Accuracy: math.Abs(in[2]),
				OK:       true,
			}
		}
	}
	return ModelResult{}
}
