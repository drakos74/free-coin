package trade

import (
	"fmt"
	"sort"

	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type OpenConfig struct {
	MinSample      int
	MinProbability float64
	OpenValue      float64
	Strategies     []TradingStrategy
}

func (c OpenConfig) evaluate(pp stats.TradeSignal) predictionsPairs {
	var pairs predictionsPairs = make([]PredictionPair, 0)
	// NOTE : we can have multiple predictions because of the number of sequences we are tracking
	// lookback and lookahead for the stats processor configs
	for _, p := range pp.Predictions {
		if p.Sample > c.MinSample {
			// add up the first predictions until we reach a reasonable Probability
			var prb float64
			values := make([]string, 0)
			for _, pv := range p.Values {
				prb += pv.Probability
				values = append(values, pv.Value)
				if prb > c.MinProbability {
					// go to next stage we what we got
					break
				}
			}
			if prb <= c.MinProbability || len(values) == 0 {
				continue
			}
			// We can have by design several strategies that will assess the prediction
			for _, strategy := range c.Strategies {
				var ttype model.Type
				open := make([]float64, 0)
				// we do have multiple predictions Values,
				// because we want to look at other predictions as well,
				// and not only the highest one potentially
				if confidence, t := strategy.exec(values, strategy.factor); t != model.NoType {
					if ttype != model.NoType && ttype != t {
						log.Warn().
							Float64("Probability", prb).
							Str("Values", fmt.Sprintf("%+v", values)).
							Msg("inconsistent prediction")
						return pairs
					} else {
						ttype = t
						open = append(open, confidence*c.OpenValue)
					}
				}
				// we create one pair for each strategy and each prediction sequence
				pairs = append(pairs, PredictionPair{
					Price:       pp.Price,
					Time:        pp.Time,
					OpenValue:   c.OpenValue,
					Strategy:    strategy.name,
					Label:       p.Label,
					Key:         p.Key,
					Values:      values,
					Probability: prb,
					Sample:      p.Sample,
					Type:        ttype,
				})
			}
		}
	}
	sort.Sort(sort.Reverse(pairs))
	return pairs
}

type TradingStrategy struct {
	name   string
	factor float64
	exec   func(vv []string, factor float64) (float64, model.Type)
}

func getVolume(price float64, value float64) float64 {
	return value / price
}
