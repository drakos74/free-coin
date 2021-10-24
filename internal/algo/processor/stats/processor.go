package stats

import (
	"math"
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
					if len(poly[2]) > 1 && math.Abs(poly[2][2]) > 0.99 &&
						len(poly[3]) > 2 && math.Abs(poly[3][3]) > 0.009 {
						// assert values
						p := 0.0
						//openType, closeType := assertType(poly)
						//o, p, err := trader.CreateOrder(
						//	fmt.Sprintf("%s_%s", cfg.Name, string(trade.Coin)),
						//	trade.Time,
						//	trade.Price,
						//	time.Duration(cfg.Duration)*time.Minute,
						//	trade.Coin,
						//	openType, closeType,
						//	1,
						//)
						if err != nil {
							u.Send(index, api.NewMessage(formatPoly(cfg, trade, poly, d, p, err)), nil)
						} else {
							u.Send(index, api.NewMessage(formatPoly(cfg, trade, poly, d, p, err)), nil)
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