package stats

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	Name = "stats-collector"
)

// Processor is the position processor main routine.
func Processor(index api.Index, shard storage.Shard, configs map[model.Coin]map[time.Duration]Config) func(u api.User, e api.Exchange) api.Processor {

	//t, err := trader.NewTrader(string(index), shard)
	//if err != nil {
	//	log.Error().Err(err).Str("processor", Name).Msg("could not init trader")
	//	return func(u api.User, e api.Exchange) api.Processor {
	//		return processor.Void(Name)
	//	}
	//}

	stats, err := newStats(shard, configs)
	// make sure we dont break the pipeline
	if err != nil {
		log.Error().Err(err).Str("processor", Name).Msg("could not init stats")
		return func(u api.User, e api.Exchange) api.Processor {
			return processor.Void(Name)
		}
	}

	return func(u api.User, e api.Exchange) api.Processor {

		go trackUserActions(u, stats)

		//trader := trader.NewExchangeTrader(t, e)

		return processor.Process(Name, func(trade *model.Trade) error {
			metrics.Observer.IncrementTrades(string(trade.Coin), Name)
			// set up the config for the coin if it s not there.
			// use "" as default ... if its missing i guess we ll fail hard at some point ...
			for duration, cfg := range stats.configs[trade.Coin] {
				k := model.NewKey(trade.Coin, duration, cfg.Name)
				// push the trade data to the stats collector window
				if buckets, poly, d, ok := stats.push(k, trade); ok {
					if len(poly[2]) > 1 && len(poly[3]) > 2 {
						p := poly[2][2] * poly[3][3]
						if p > 0 {
							log.Debug().
								Time("stamp", trade.Time).
								Str("price", fmt.Sprintf("%v", trade.Price)).
								Str("coin", string(trade.Coin)).
								Str("2", fmt.Sprintf("%+v", poly[2][2])).
								Str("3", fmt.Sprintf("%+v", poly[3][3])).
								Str("p", fmt.Sprintf("%+v", poly[2][2]*poly[3][3])).
								Msg("poly")
							// create a trade signal
							signal := Signal{
								Density:  d,
								Coin:     trade.Coin,
								Factor:   p,
								Type:     model.SignedType(poly[2][2]),
								Price:    trade.Price,
								Time:     trade.Time,
								Duration: duration,
								Segments: cfg.Model.Stats[0].LookAhead + cfg.Model.Stats[0].LookBack,
							}
							if cfg.Threshold >= 0 && signal.Filter(cfg.Threshold) {
								u.Send(index, api.NewMessage(formatSignal(signal, cfg.Threshold)), nil)
							}
							if cfg.Notify.Stats {
								// TODO : enable only for testing
								ss, err := json.Marshal(signal)
								if err != nil {
									log.Warn().Msg("error marshalling signal")
								}
								u.Send(index, api.NewMessage(string(ss)), nil)
							}
						}
					}
					values, indicators, last := extractFromBuckets(buckets, group(getPriceRatio, cfg.Order.Exec))
					// count the occurrences
					// TODO : give more weight to the more recent values that come in
					// TODO : old values might destroy our statistics and be irrelevant
					predictions, status := stats.add(k, values[0][len(values[0])-1])
					if trade.Live {
						if trade.Signals == nil {
							trade.Signals = make([]model.Signal, 0)
						}
						aggregateStats := coinmath.NewAggregateStats(indicators)
						// Note there is one trade signal per key (coin,duration) pair
						trade.Signals = append(trade.Signals, model.Signal{
							Type: "TradeSignal",
							Value: TradeSignal{
								SignalEvent: SignalEvent{
									ID:    uuid.New().String(),
									Key:   k,
									Price: trade.Price,
									Time:  trade.Time,
								},
								Predictions:    predictions,
								AggregateStats: aggregateStats,
							},
						})
						if u != nil && cfg.Notify.Stats {
							// TODO : add tests for this
							u.Send(index,
								api.NewMessage(formatΗΜΜMessage(last, values, aggregateStats, predictions, status, trade, cfg)).
									ReferenceTime(trade.Time), nil)
						}
						// TODO : expose in metrics
					}
				}
			}
			return nil
		})
	}
}

func assertType(poly map[int][]float64) (open, close model.Type) {

	p2 := poly[2][2]
	p3 := p2
	if len(poly[3]) > 2 {
		p3 = poly[3][3] * 100
	}

	if p2 > 0.99 {
		if p3 > 0.99 {
			// open new for sure
			return model.Buy, model.Buy
		} else if p3 < -0.99 {
			return 0, model.Sell
		}
	} else if p2 < -0.99 {
		if p3 < -0.99 {
			return model.Sell, model.Sell
		} else if p3 > 0.99 {
			return 0, model.Buy
		}
	}
	return 0, 0
}
