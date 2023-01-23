package ml

import (
	"fmt"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/algo/processor/ml/net"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
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
func Processor(index api.Index, shard storage.Shard, registry storage.EventRegistry, config *mlmodel.Config, networks ...net.ConstructNetwork) func(u api.User, e api.Exchange) api.Processor {
	col, err := newCollector(4, shard, nil, config)
	// make sure we don't break the pipeline
	if err != nil {
		log.Error().Err(err).Str("processor", Name).Msg("could not init processor")
		return func(u api.User, e api.Exchange) api.Processor {
			return processor.Void(Name)
		}
	}

	//network := math.NewML(net)

	benchmarks := mlmodel.NewBenchmarks()

	ds := net.NewDataSets(shard, net.MultiNetworkConstructor(networks...))

	strategy := newStrategy(config, ds)
	return func(u api.User, e api.Exchange) api.Processor {
		wallet, err := trader.SimpleTrader(string(index), shard, registry, trader.Settings{
			OpenValue:      config.Position.OpenValue,
			TakeProfit:     config.Position.TakeProfit,
			StopLoss:       config.Position.StopLoss,
			TrackingConfig: config.Position.TrackingConfig,
		}, e, u)
		if err != nil {
			log.Error().Err(err).Str("processor", Name).Msg("processor in void state")
			return processor.NoProcess(Name)
		}

		// init the user interactions
		go trackUserActions(index, u, col, strategy, wallet, benchmarks, config)
		u.Send(index, api.NewMessage(fmt.Sprintf("starting processor ... %s", formatConfig(*config))), nil)

		// process the collector vectors
		go func(col *collector) {
			for vv := range col.vectors {
				coin := string(vv.Meta.Key.Coin)
				duration := vv.Meta.Key.Duration.String()
				metrics.Observer.IncrementEvents(coin, duration, "poly", Name)
				configSegments := config.GetSegments(vv.Meta.Key.Coin, vv.Meta.Key.Duration)
				for key, segmentConfig := range configSegments {
					// do our training here ...
					if segmentConfig.Model.Features > 0 && key.Match(vv.Meta.Key.Coin) {
						metrics.Observer.IncrementEvents(coin, duration, "train", Name)
						if set, ok := ds.Push(key, vv, segmentConfig.Model); ok {
							metrics.Observer.IncrementEvents(coin, duration, "train_buffer", Name)
							result, tt := set.Train()
							signal := mlmodel.Signal{
								Key:    key,
								Detail: result.Detail,
								Time:   vv.Meta.Tick.Range.To.Time,
								Price:  vv.Meta.Tick.Range.To.Price,
								Type:   result.Type,
								//Spectrum:  coin_math.FFT(vv.YY),
								//Buffer:    vv.YY,
								Gap:       result.Gap,
								Precision: result.Accuracy,
								Trend:     result.Trend,
								Weight:    segmentConfig.Trader.Weight,
								Live:      segmentConfig.Trader.Live,
							}
							if ok && signal.Type != model.NoType {
								if s, k, open, ok := strategy.eval(vv.Meta.Tick, signal, config); ok {
									_, ok, action, err := wallet.CreateOrder(k, s.Time, s.Price, s.Type, open, 0, trader.SignalReason, s.Detail.Type, s.Live)
									if err != nil {
										log.Error().Str("signal", fmt.Sprintf("%+v", s)).Err(err).Msg("error creating order")
									} else if !ok {
										log.Debug().Str("action", fmt.Sprintf("%+v", action)).Str("signal", fmt.Sprintf("%+v", s)).Bool("open", open).Bool("ok", ok).Err(err).Msg("error submitting order")
									}
									u.Send(index, api.NewMessage(formatSignal(config.Option.Log, s, action, err, ok)).AddLine(fmt.Sprintf("%s", emoji.MapToValid(s.Live))), nil)
								}
								log.Info().
									Str("coin", string(key.Coin)).
									Floats64("features", result.Features).
									Str("detail", fmt.Sprintf("%+v", result.Detail.Type)).
									Str("signal", fmt.Sprintf("%+v", signal)).
									Msg("features")
							}
							// whatever happened , lets benchmark it
							if config.Option.Benchmark {
								for k, res := range tt {
									kk := key
									kk.Strategy = k.Type
									if res.Reset {
										benchmarks.Reset(kk.Coin, kk)
									} else {
										s := signal
										s.Type = res.Type
										report, ok, _ := benchmarks.Add(kk, vv.Meta.Tick, s, config)
										if ok {
											set.Eval(k, report)
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
					pp, profit, trend, _ := wallet.Update(config.Option.Trace, tradeSignal, config.Position.TrackingConfig)
					if len(pp) > 0 {
						for k, p := range pp {
							reason := trader.VoidReasonClose
							if p.PnL > 0 {
								reason = trader.TakeProfitReason
							} else if p.PnL < 0 {
								reason = trader.StopLossReason
							}
							_, ok, action, err := wallet.CreateOrder(k, tradeSignal.Meta.Time, tradeSignal.Tick.Price, p.Type.Inv(), false, p.Volume, reason, p.Stats.Strategy, p.Live)
							if err != nil || !ok {
								log.Error().Err(err).Bool("ok", ok).Msg("could not close position")
							} else if floats.Sum(profit) < 0 {
								ok := strategy.reset(k)
								if !ok {
									log.Error().Str("Index", k.ToString()).Msg("could not reset signal")
								}
							}
							u.Send(index, api.NewMessage(formatAction(config.Option.Log, action, trend[k], err, ok)).AddLine(fmt.Sprintf("%s", emoji.MapToValid(p.Live))), nil)
						}
					} else if len(trend) > 0 && config.Option.Trace[string(tradeSignal.Coin)] {
						u.Send(index, api.NewMessage(fmt.Sprintf("%s %s", formatTime(tradeSignal.Tick.Time), tradeSignal.Coin)).AddLine(formatTrend(trend)), nil)
					}
					// print the reports for trace reasons
					//for k, report := range reports {
					//	if config.Option.Trace[string(k.Coin)] {
					//		u.Send(index, api.NewMessage(formatTrendReport(config.Option.Log, k, report)), nil)
					//	}
					//}
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
		}, processor.Deriv())
	}
}
