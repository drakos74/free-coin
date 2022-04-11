package ml

import (
	"fmt"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	coin_math "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/trader"
	"github.com/rs/zerolog/log"
	"gonum.org/v1/gonum/floats"
)

const (
	Name = "ml-network"
)

// Processor is the position processor main routine.
func Processor(index api.Index, shard storage.Shard, registry storage.EventRegistry, networkConstructor func() Network, config *Config) func(u api.User, e api.Exchange) api.Processor {
	col, err := newCollector(2, shard, nil, config)
	// make sure we dont break the pipeline
	if err != nil {
		log.Error().Err(err).Str("processor", Name).Msg("could not init processor")
		return func(u api.User, e api.Exchange) api.Processor {
			return processor.Void(Name)
		}
	}

	//network := math.NewML(net)

	benchmarks := newBenchmarks()

	ds := newDataSets(networkConstructor)

	strategy := newStrategy(config, ds)
	return func(u api.User, e api.Exchange) api.Processor {
		wallet, err := trader.SimpleTrader(string(index), shard, registry, trader.Settings{
			OpenValue:      config.Position.OpenValue,
			TakeProfit:     config.Position.TakeProfit,
			StopLoss:       config.Position.StopLoss,
			TrackingConfig: config.Position.TrackingConfig,
		}, e)
		if err != nil {
			log.Error().Err(err).Str("processor", Name).Msg("processor in void state")
			return processor.NoProcess(Name)
		}

		go trackUserActions(index, u, col, strategy, wallet, benchmarks, config)

		u.Send(index, api.NewMessage(fmt.Sprintf("starting processor ... %s", formatConfig(*config))), nil)

		// process the collector vectors
		go func(col *collector) {
			for vv := range col.vectors {
				coin := string(vv.meta.key.Coin)
				duration := vv.meta.key.Duration.String()
				metrics.Observer.IncrementEvents(coin, duration, "poly", Name)
				configSegments := config.segments(vv.meta.key.Coin, vv.meta.key.Duration)
				for key, segmentConfig := range configSegments {
					// do our training here ...
					if segmentConfig.Model.Features > 0 && key.Match(vv.meta.key.Coin) {
						metrics.Observer.IncrementEvents(coin, duration, "train", Name)
						if set, ok := ds.push(key, vv, segmentConfig.Model); ok {
							metrics.Observer.IncrementEvents(coin, duration, "train_buffer", Name)
							result, tt := set.network.Train(segmentConfig.Model, set)
							signal := Signal{
								Key:       key,
								Detail:    result.key,
								Time:      vv.meta.tick.Time,
								Price:     vv.meta.tick.Price,
								Type:      result.t,
								Spectrum:  coin_math.FFT(vv.yy),
								Buffer:    vv.yy,
								Precision: result.acc,
								Weight:    segmentConfig.Trader.Weight,
								Live:      segmentConfig.Trader.Live,
							}
							if ok && signal.Type != model.NoType {
								if s, k, open, ok := strategy.eval(vv.meta.tick, signal, config); ok {
									_, ok, action, err := wallet.CreateOrder(k, s.Time, s.Price, s.Type, open, 0, trader.SignalReason, s.Live)
									if err != nil {
										log.Error().Str("signal", fmt.Sprintf("%+v", s)).Err(err).Msg("error creating order")
									} else if !ok {
										log.Debug().Str("action", fmt.Sprintf("%+v", action)).Str("signal", fmt.Sprintf("%+v", s)).Bool("open", open).Bool("ok", ok).Err(err).Msg("error submitting order")
									}
									u.Send(index, api.NewMessage(formatSignal(s, action, err, ok)).AddLine(fmt.Sprintf("%s", emoji.MapToValid(s.Live))), nil)
								}
							}
							// whatever happened , lets benchmark it
							if config.Option.Benchmark {
								for k, res := range tt {
									kk := key
									kk.Strategy = k
									if res.reset {
										benchmarks.reset(kk.Coin, kk)
									} else {
										s := signal
										s.Type = res.t
										report, ok, _ := benchmarks.add(kk, vv.meta.tick, s, config)
										if ok {
											set.network.Eval(k, report)
										}
									}
								}
							}
						}
					}
				}
			}
		}(col)

		return processor.ProcessBufferedWithClose(Name, config.Buffer.Interval, func(tradeSignal *model.TradeSignal) error {
			coin := string(tradeSignal.Coin)
			f, _ := strconv.ParseFloat(tradeSignal.Meta.Time.Format("0102.1504"), 64)
			start := time.Now()
			metrics.Observer.NoteLag(f, coin, Name, "batch")
			metrics.Observer.IncrementTrades(coin, Name, "batch")
			col.push(tradeSignal)
			if live, first := strategy.isLive(tradeSignal.Coin, tradeSignal.Tick); live || config.Option.Debug {
				startStrategy := time.Now()
				if first {
					u.Send(index, api.NewMessage(fmt.Sprintf("%s strategy going live for %s",
						tradeSignal.Meta.Time.Format(time.Stamp), tradeSignal.Coin)), nil)
				}
				if tradeSignal.Meta.Live || config.Option.Debug {
					metrics.Observer.NoteLag(f, coin, Name, "process")
					pp, profit, trend := wallet.Update(tradeSignal)
					if len(pp) > 0 {
						for k, p := range pp {
							reason := trader.VoidReasonClose
							if p.PnL > 0 {
								reason = trader.TakeProfitReason
							} else if p.PnL < 0 {
								reason = trader.StopLossReason
							}
							_, ok, action, err := wallet.CreateOrder(k, tradeSignal.Meta.Time, tradeSignal.Tick.Price, p.Type.Inv(), false, p.Volume, reason, p.Live)
							if err != nil || !ok {
								log.Error().Err(err).Bool("ok", ok).Msg("could not close position")
							} else if floats.Sum(profit) < 0 {
								ok := strategy.reset(k)
								if !ok {
									log.Error().Str("key", k.ToString()).Msg("could not reset signal")
								}
							}
							u.Send(index, api.NewMessage(formatAction(action, profit, trend, err, ok)).AddLine(fmt.Sprintf("%s", emoji.MapToValid(p.Live))), nil)
						}
					} else if len(trend) > 0 {
						u.Send(index, api.NewMessage(formatTrend(tradeSignal, trend)), nil)
					}
				}
				strategyDuration := time.Now().Sub(startStrategy).Seconds()
				metrics.Observer.TrackDuration(strategyDuration, coin, Name, "strategy")
			}
			duration := time.Now().Sub(start).Seconds()
			metrics.Observer.TrackDuration(duration, coin, Name, "process")
			return nil
		}, func() {
			//for c, aa := range wallet.Actions() {
			//	fmt.Printf("c = %+v\n", c)
			//	for _, a := range aa {
			//		fmt.Printf("a = %+v\n", a)
			//	}
			//}
			//pp := make([][]client.Report, 0)
			//for c, dd := range benchmarks.assess() {
			//	fmt.Printf("c = %+v\n", c)
			//	for d, tt := range dd {
			//		fmt.Printf("d = %+v\n", d)
			//		pp = append(pp, tt)
			//	}
			//}
		})
	}
}
