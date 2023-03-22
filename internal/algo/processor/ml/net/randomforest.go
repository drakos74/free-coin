package net

import (
	"fmt"
	"math"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type RandomForest struct {
	SingleNetwork
	forest *ml.RandomForest
	cfg    mlmodel.Model
	debug  bool
	tmpKey string
}

func ConstructRandomForest(debug bool) func(cfg mlmodel.Model) Network {
	return func(cfg mlmodel.Model) Network {
		// NOTE : by default randomise the construction
		config := cfg.Evolve()
		return NewRandomForest(config, coinmath.String(10), debug)
	}
}

func NewRandomForest(cfg mlmodel.Model, key string, debug bool) *RandomForest {
	return &RandomForest{
		SingleNetwork: NewSingleNetwork(cfg),
		forest:        ml.NewForest(1000),
		cfg:           cfg,
		debug:         debug,
		tmpKey:        key,
	}
}

func (r *RandomForest) Train(ds *Dataset) ModelResult {
	xx := make([][]float64, 0)
	yy := make([]int, 0)

	newX := make([]float64, 0)
	for _, v := range ds.Vectors {
		if len(v.PrevIn) != r.config.Features {
			continue
		}
		xx = append(xx, v.PrevIn)
		y := -1
		for i, o := range v.PrevOut {
			if o == 1 {
				y = i
			}
		}
		yy = append(yy, y)
		newX = v.NewIn
	}

	var t model.Type
	var p []float64
	var acc float64
	var features []float64
	if len(xx) > 0 && len(yy) > 0 {
		acc, features = r.forest.Train(xx, yy)
	}

	if len(newX) == r.config.Features {
		p = r.forest.Predict(newX)
	} else {
		log.Info().
			Str("coin", string(ds.Coin)).
			Str("duration", fmt.Sprintf("%+v", ds.Duration)).
			Int("xx", len(xx)).
			Int("yy", len(yy)).
			Str("newX", fmt.Sprintf("%+v", newX)).
			Str("config", fmt.Sprintf("%+v", r.config)).
			Msg("cannot predict random forest")
	}

	gap := 0.0
	if len(p) >= 3 && acc > r.cfg.PrecisionThreshold {
		if p[0] > p[2] {
			t = model.Buy
		} else if p[2] > p[0] {
			t = model.Sell
		}
		gap = math.Abs(p[0] - p[2])
		// TODO : pass this from config
		//if gap < 0.25 {
		//	// cancel the signal
		//	t = model.NoType
		//}
	}

	result := ModelResult{
		Detail: mlmodel.Detail{
			Type: networkType(r),
			Hash: r.tmpKey,
		},
		Features:           newX,
		FeaturesImportance: features,
		Type:               t,
		Accuracy:           acc,
		Gap:                gap,
		OK:                 true,
	}

	return result
}
