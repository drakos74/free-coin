package ml

import (
	"fmt"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	coin_math "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/trader"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/rs/zerolog/log"
	"gonum.org/v1/gonum/floats"
)

const (
	Name = "ml-network"
)

// Processor is the position processor main routine.
func Processor(index api.Index, shard storage.Shard, registry storage.EventRegistry, _ *ff.Network, config *Config) func(u api.User, e api.Exchange) api.Processor {
	collector, err := newCollector(2, shard, nil, config)
	// make sure we dont break the pipeline
	if err != nil {
		log.Error().Err(err).Str("processor", Name).Msg("could not init processor")
		return func(u api.User, e api.Exchange) api.Processor {
			return processor.Void(Name)
		}
	}

	//network := math.NewML(net)

	benchmarks := newBenchmarks()

	strategy := newStrategy(config.Segments)
	return func(u api.User, e api.Exchange) api.Processor {
		wallet, err := trader.SimpleTrader(string(index), shard, registry, trader.Settings{
			OpenValue:  config.Position.OpenValue,
			TakeProfit: config.Position.TakeProfit,
			StopLoss:   config.Position.StopLoss,
		}, e)
		if err != nil {
			log.Error().Err(err).Str("processor", Name).Msg("processor in void state")
			return processor.NoProcess(Name)
		}

		go trackUserActions(index, u, collector, strategy, wallet, benchmarks, config)

		u.Send(index, api.NewMessage(fmt.Sprintf("starting processor ... %s", formatConfig(*config))), nil)

		ds := newDataSets()
		// process the collector vectors
		for vv := range collector.vectors {
			metrics.Observer.IncrementEvents(string(vv.meta.coin), vv.meta.duration.String(), "poly", Name)
			configSegments := config.segments(vv.meta.coin, vv.meta.duration)
			signal := Signal{}
			for key, segmentConfig := range configSegments {
				// do our training here ...
				if segmentConfig.Model.Features > 0 {
					metrics.Observer.IncrementEvents(string(vv.meta.coin), vv.meta.duration.String(), "train", Name)
					if set, ok := ds.push(key, vv, segmentConfig); ok {
						metrics.Observer.IncrementEvents(string(vv.meta.coin), vv.meta.duration.String(), "train_buffer", Name)
						if t, acc, ok := set.train(segmentConfig.Model, config.Option.Test); ok {
							signal = Signal{
								Key:       key,
								Time:      vv.meta.tick.Time,
								Price:     vv.meta.tick.Price,
								Type:      t,
								Spectrum:  coin_math.FFT(vv.yy),
								Buffer:    vv.yy,
								Precision: acc,
								Weight:    segmentConfig.Trader.Weight,
							}
							if config.Option.Benchmark {
								benchmarks.add(key, vv.meta.tick, signal, config)
							}
						}
					}
				}
			}
			if s, k, open, ok := strategy.eval(vv.meta.tick, signal, config); ok {
				if config.Option.Test {
					u.Send(index,
						api.NewMessage(formatSignal(s, trader.Event{}, err, ok)).
							AddLine("test"),
						nil)
					continue
				}
				_, ok, action, err := wallet.CreateOrder(k, s.Time, s.Price, s.Type, open, 0, trader.SignalReason)
				if err != nil {
					log.Error().Str("signal", fmt.Sprintf("%+v", s)).Err(err).Msg("error creating order")
				} else if ok && config.Option.Debug {
					// track the raw signals for debug purposes
					u.Send(index, api.NewMessage(encodeMessage(s)), nil)
				} else if !ok {
					log.Debug().Str("action", fmt.Sprintf("%+v", action)).Str("signal", fmt.Sprintf("%+v", s)).Bool("open", open).Bool("ok", ok).Err(err).Msg("error submitting order")
				}
				u.Send(index, api.NewMessage(formatSignal(s, action, err, ok)), nil)
			}
		}

		return processor.ProcessBufferedWithClose(Name, time.Minute, func(tradeSignal *model.TradeSignal) error {
			coin := string(tradeSignal.Coin)
			f, _ := strconv.ParseFloat(tradeSignal.Meta.Time.Format("0102.1504"), 64)
			start := time.Now()
			metrics.Observer.NoteLag(f, coin, Name, "batch")
			metrics.Observer.IncrementTrades(coin, Name, "batch")
			collector.push(tradeSignal)
			if live, first := strategy.isLive(tradeSignal.Coin, tradeSignal.Tick); live || config.Option.Debug {
				startStrategy := time.Now()
				if first {
					u.Send(index, api.NewMessage(fmt.Sprintf("%s strategy going live for %s",
						tradeSignal.Meta.Time.Format(time.Stamp), tradeSignal.Coin)), nil)
				}
				if tradeSignal.Meta.Live || config.Option.Debug {
					metrics.Observer.NoteLag(f, coin, Name, "process")
					pp, profit := wallet.Update(tradeSignal)
					if len(pp) > 0 {
						for k, p := range pp {
							reason := trader.VoidReasonClose
							if p.PnL > 0 {
								reason = trader.TakeProfitReason
							} else if p.PnL < 0 {
								reason = trader.StopLossReason
							}
							if config.Option.Test {
								u.Send(index,
									api.NewMessage(formatAction(trader.Event{}, profit, err, false)).
										AddLine("test"),
									nil)
								continue
							}
							_, ok, action, err := wallet.CreateOrder(k, tradeSignal.Meta.Time, tradeSignal.Tick.Price, p.Type.Inv(), false, p.Volume, reason)
							if err != nil || !ok {
								log.Error().Err(err).Bool("ok", ok).Msg("could not close position")
							} else if floats.Sum(profit) < 0 {
								ok := strategy.reset(k)
								if !ok {
									log.Error().Str("key", k.ToString()).Msg("could not reset signal")
								}
							}
							u.Send(index, api.NewMessage(formatAction(action, profit, err, ok)), nil)
						}
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
