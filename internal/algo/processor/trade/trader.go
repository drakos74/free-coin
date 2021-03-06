package trade

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/buffer"

	"github.com/drakos74/free-coin/internal/algo/processor/stats"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type trader struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	logger  storage.Registry
	lock    sync.RWMutex
	configs map[model.Coin]map[time.Duration]processor.Config
	logs    map[string]struct{}
}

func newTrader(registry storage.Registry, configs map[model.Coin]map[time.Duration]processor.Config) *trader {
	return &trader{
		logger:  registry,
		lock:    sync.RWMutex{},
		configs: configs,
		logs:    make(map[string]struct{}),
	}
}

func (tr *trader) get(k processor.Key) (processor.Config, bool) {
	tr.lock.RLock()
	defer tr.lock.RUnlock()
	cfg, ok := tr.configs[k.Coin][k.Duration]
	return cfg, ok
}

func (tr *trader) getAll(c model.Coin) map[time.Duration]processor.Config {
	tr.lock.RLock()
	defer tr.lock.RUnlock()
	configs := make(map[time.Duration]processor.Config)
	for d, cfg := range tr.configs[c] {
		configs[d] = cfg
	}
	return configs
}

// TODO : re-enable set loogic at some point
//func (tr *trader) set(k processor.Key, probability float64, sample int) (time.Duration, OpenConfig) {
//	tr.init(k)
//	tr.lock.Lock()
//	defer tr.lock.Unlock()
//	cfg := tr.configs[k.Coin][k.Duration]
//	cfg.MinProbability = probability
//	cfg.MinSample = sample
//	tr.configs[k.Coin][k.Duration] = cfg
//	return k.Duration, tr.configs[k.Coin][k.Duration]
//}

func evaluate(pp stats.TradeSignal, strategies []processor.Strategy) predictionsPairs {
	var pairs predictionsPairs = make([]PredictionPair, 0)
	// NOTE : we can have multiple predictions because of the number of sequences we are tracking
	// lookback and lookahead for the stats processor configs
	for _, prediction := range pp.Predictions {
		for _, strategy := range strategies {
			if pair, ok := doEvaluate(prediction, strategy); ok {
				pair.Price = pp.Price
				pair.Time = pp.Time
				pairs = append(pairs, pair)
			}
		}
	}
	sort.Sort(sort.Reverse(pairs))
	return pairs
}

func doEvaluate(prediction buffer.Predictions, strategy processor.Strategy) (PredictionPair, bool) {
	// only continue if the prediction duration matches with the strategy
	if prediction.Sample > strategy.Sample {
		// add up the first predictions until we reach a reasonable Probability
		var prb float64
		values := make([]buffer.Sequence, 0)
		for _, pv := range prediction.Values {
			prb += pv.Probability
			values = append(values, pv.Value)
			if prb > strategy.Probability {
				// go to next stage we what we got
				break
			}
		}
		if prb <= strategy.Probability || len(values) == 0 {
			return PredictionPair{}, false
		}
		// We can have by design several strategies that will assess the prediction

		var ttype model.Type
		var tconfidence float64
		// we do have multiple predictions Values,
		// because we want to look at other predictions as well,
		// and not only the highest one potentially
		if confidence, t := getStrategy(strategy.Name, strategy.Threshold)(values, strategy.Factor); t != model.NoType {
			if ttype != model.NoType && t != ttype {
				log.Warn().
					Float64("Probability", prb).
					Str("Values", fmt.Sprintf("%+v", values)).
					Msg("inconsistent prediction")
				// We cant be conclusive about the strategy based on the prediciton data
				return PredictionPair{}, false
			} else if t != model.NoType {
				ttype = t
				tconfidence = confidence
			}
		}
		// we create one pair for each strategy and each prediction sequence
		return PredictionPair{
			Strategy:    strategy.Name,
			Confidence:  tconfidence,
			Open:        strategy.Open.Value,
			Label:       prediction.Label,
			Key:         prediction.Key,
			Values:      values,
			Probability: prb,
			Sample:      prediction.Sample,
			Type:        ttype,
		}, ttype != model.NoType
	}
	return PredictionPair{}, false
}

func getStrategy(name string, threshold float64) TradingStrategy {
	switch name {
	case processor.NumericStrategy:
		return func(vv []buffer.Sequence, factor float64) (float64, model.Type) {
			// note : each element of the map could contain multiple prediction Values
			// gather them all together though ... with some weighting on the index
			t := model.NoType
			value := 0.0
			s := 0.0
			// make it simple if we have one prediction
			for _, y := range vv {
				ww := y.Values()
				l := len(ww)
				for w, v := range ww {
					i, err := strconv.ParseFloat(v, 64)
					if err != nil {
						return factor, t
					}
					g := float64(l-w) * i
					value += g
					s++
				}
			}
			x := value / s
			t = model.SignedType(x)
			if math.Abs(x) >= threshold {
				return factor, t
			}
			return 0, model.NoType
		}
	}
	return func(vv []buffer.Sequence, factor float64) (float64, model.Type) {
		return 0.0, model.NoType
	}
}

type TradingStrategy func(vv []buffer.Sequence, factor float64) (float64, model.Type)

func getVolume(price float64, value float64) float64 {
	return value / price
}
