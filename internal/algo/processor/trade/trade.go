package trade

import (
	"fmt"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
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
	Name = "trade-execution"
)

// Processor is the position processor main routine.
func Processor(index api.Index, shard storage.Shard, registry storage.EventRegistry, strategy *processor.Strategy) func(u api.User, e api.Exchange) api.Processor {

	// make sure we don't break the pipeline
	//if err != nil {
	//	log.Error().Err(err).Str("processor", Name).Msg("could not init processor")
	//	return func(u api.User, e api.Exchange) api.Processor {
	//		return processor.Void(Name)
	//	}
	//}

	config := strategy.Config()

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
		go trackUserActions(index, u, strategy, wallet)
		u.Send(index, api.NewMessage(fmt.Sprintf("%s starting processor ... %s", Name, formatConfig(config))), nil)

		return processor.ProcessBufferedWithClose(Name, config.Buffer.Interval, true, func(tradeSignal *model.TradeSignal) error {
			coin := string(tradeSignal.Coin)
			f, _ := strconv.ParseFloat(tradeSignal.Meta.Time.Format("20060102.1504"), 64)
			start := time.Now()
			metrics.Observer.NoteLag(f, coin, Name, "batch")
			metrics.Observer.IncrementTrades(coin, Name, "batch")
			// TODO : highlight the data flow better
			// check conditions for closing open positions
			if live, first := strategy.IsLive(tradeSignal.Coin, tradeSignal.Tick); live || config.Option.Debug {
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
							// for statistics reasons
							reason, _ := trader.Assess(p.PnL)
							_, ok, action, err := wallet.CreateOrder(k, tradeSignal.Meta.Time, tradeSignal.Tick.Price, p.Type.Inv(), false, p.Volume, reason, p.Live, nil)
							if err != nil || !ok {
								log.Error().Err(err).Bool("ok", ok).Msg("could not close position")
							} else if floats.Sum(profit) < 0 {
								ok := strategy.Reset(k)
								if !ok {
									log.Error().Str("Index", k.ToString()).Msg("could not reset signal")
								}
							}
							action.SourceTime = p.OpenTime
							u.Send(index, api.NewMessage(formatAction(config.Option.Log, action, trend[k], err, ok)).
								//AddLine(fmt.Sprintf(formatDecision(p.Decision))).
								AddLine(fmt.Sprintf("%s", emoji.MapToValid(p.Live))), nil)
						}
					} else if len(trend) > 0 && (config.Option.Trace[string(tradeSignal.Coin)] || config.Option.Trace[string(model.AllCoins)]) {
						u.Send(index, api.NewMessage(fmt.Sprintf("%s %s", formatTime(tradeSignal.Tick.Time), tradeSignal.Coin)).AddLine(formatTrend(trend)), nil)
					}
				}
				strategyDuration := time.Now().Sub(startStrategy).Seconds()
				metrics.Observer.TrackDuration(strategyDuration, coin, Name, "strategy")
			}
			duration := time.Now().Sub(start).Seconds()
			metrics.Observer.TrackDuration(duration, coin, Name, "process")
			return nil
		}, func() {
			// something went terribly wrong in this processor or in the pipeline in general
		}, processor.Deriv())
	}
}
