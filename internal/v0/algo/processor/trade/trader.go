package trade

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/buffer"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type trader struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	registry storage.Registry
	lock     sync.RWMutex
	configs  map[model.Coin]map[time.Duration]processor.Config
	logs     map[string]struct{}
}

func newTrader(registry storage.Registry, configs map[model.Coin]map[time.Duration]processor.Config) *trader {
	return &trader{
		registry: registry,
		lock:     sync.RWMutex{},
		configs:  configs,
		logs:     make(map[string]struct{}),
	}
}

func (tr *trader) get(k model.Key) (processor.Config, bool) {
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

// TODO : re-enable set logic at some point
//func (tr *trader) set(k model.Key, probability float64, sample int) (time.Duration, OpenConfig) {
//	tr.init(k)
//	tr.lock.Lock()
//	defer tr.lock.Unlock()
//	cfg := tr.configs[k.Coin][k.Duration]
//	cfg.MinProbability = probability
//	cfg.MinSample = sample
//	tr.configs[k.Coin][k.Duration] = cfg
//	return k.Duration, tr.configs[k.Coin][k.Duration]
//}

func (tr *trader) evaluate(pp TradeSignal, strategy processor.Strategy) predictionsPairs {
	var pairs predictionsPairs = make([]PredictionPair, 0)
	// NOTE : we can have multiple predictions because of the number of sequences we are tracking
	// lookback and lookahead for the stats processor configs
	for _, prediction := range pp.Predictions {
		executor := tr.getStrategy(strategy.Name)
		if values, probability, confidence, ttype := executor(pp.SignalEvent, prediction, strategy); ttype != model.NoType {
			pair := PredictionPair{
				SignalID:    pp.ID,
				Price:       pp.Price,
				Time:        pp.Time,
				Strategy:    strategy,
				Confidence:  confidence,
				Label:       prediction.Label,
				Key:         prediction.Key,
				Values:      values,
				Probability: probability,
				Sample:      prediction.Sample,
				Type:        ttype,
			}
			pairs = append(pairs, pair)
		}
	}
	sort.Sort(sort.Reverse(pairs))
	return pairs
}

func (tr *trader) register(k storage.K, event StrategyEvent) {
	err := tr.registry.Add(k, event)
	if err != nil {
		log.Error().
			Err(err).
			Str("event", fmt.Sprintf("%+v", event)).
			Msg("could not save strategy event")
	}
}

func (tr *trader) getStrategy(name string) ExecStrategy {
	switch name {
	case processor.NumericStrategy:
		return func(signal SignalEvent, predictions buffer.Predictions, strategy processor.Strategy) (values []buffer.Sequence, probability float64, confidence float64, ttype model.Type) {
			// only continue if the prediction duration matches with the strategy
			cid := signal.Key.ToString()
			key := strategyKey(string(signal.Key.Coin))
			event := StrategyEvent{
				Time:     signal.Time,
				Coin:     signal.Key.Coin,
				Strategy: cid,
				Sample: Sample{
					Strategy:    strategy.Sample,
					Predictions: predictions.Sample,
				},
			}
			if predictions.Sample > strategy.Sample {
				event.Sample.Valid = true
				// add up the first predictions until we reach a reasonable Probability
				var prb float64
				values = make([]buffer.Sequence, 0)
				for _, pv := range predictions.Values {
					acc := strategy.DecayFactor*pv.EMP + (1-strategy.DecayFactor)*pv.Probability
					prb += acc
					values = append(values, pv.Value)
					if prb > strategy.Probability {
						// go to next stage we what we got
						break
					}
				}
				event.Probability.Strategy = strategy.Probability
				event.Probability.Predictions = prb
				if prb <= strategy.Probability || len(values) == 0 {
					return nil, 0, 0, model.NoType
				}
				// We can have by design several strategies that will assess the prediction
				event.Probability.Valid = true
				event.Probability.Values = values
				var value float64
				var s float64
				for _, y := range values {
					ww := y.Values()
					l := len(ww)
					for w, v := range ww {
						i, err := strconv.ParseFloat(v, 64)
						if math.Abs(i) > 10 {
							i = 0
						}
						if err != nil {
							log.Error().Err(err).Strs("sequence", ww).Msg("could not parse prediction sequence")
							return nil, 0, 0, model.NoType
						}
						// give some weight to the nearest prediction value
						a := float64(l - w)
						g := a * i
						value += g
						s += a
					}
				}
				x := value / s
				ttype = model.SignedType(x)
				event.Result.Sum = value
				event.Result.Count = s
				event.Result.Threshold = strategy.Threshold
				event.Result.Rating = x
				event.Result.Type = ttype
				if math.Abs(x) >= strategy.Threshold {
					// compute the confidence factor, e.g. how much we are certain of the prediction
					confidence = 1 + strategy.Factor*(prb-strategy.Probability)
					event.Result.Confidence = confidence
					log.Info().
						Float64("confidence", confidence).
						Float64("probability", prb).
						Time("time", signal.Time).
						Str("values", fmt.Sprintf("%+v", values)).
						Float64("x", x).
						Float64("s", s).
						Float64("v", value).
						Msg("pick strategy")
					tr.register(key, event)
					return values, prb, confidence, ttype
				} else {
					tr.register(key, event)
				}
			}
			return nil, 0, 0, model.NoType
		}
	}
	return func(signal SignalEvent, predictions buffer.Predictions, strategy processor.Strategy) ([]buffer.Sequence, float64, float64, model.Type) {
		return nil, 0, 0, model.NoType
	}
}

type ExecStrategy func(signal SignalEvent, predictions buffer.Predictions, strategy processor.Strategy) ([]buffer.Sequence, float64, float64, model.Type)

func getVolume(price float64, value float64, confidence float64) float64 {
	f := confidence * value / price
	return f
}
