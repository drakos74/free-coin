package trader

import (
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/concurrent"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

// ProcessorName is the name for this processor.
const ProcessorName = "trader"

// Trade defines the processing logic given the order signal.
func Trade(id string, shard storage.Shard, eventRegistry storage.EventRegistry, client api.Exchange, user api.User, config Config) algo.SignalProcessor {
	registry, err := eventRegistry(id)
	if err != nil {
		log.Error().Err(err).
			Str("user", id).
			Msg("could not create registry")
		return algo.Void()
	}

	// init trader and related actions
	trader, err := newTrader(id, client, shard, config.Settings)
	if err != nil {
		log.Error().Err(err).
			Str("user", id).
			Str("processor", ProcessorName).
			Msg("could not start processor")
		return algo.Void()
	}
	concurrent.Async(func() { trader.portfolio(client, user) })
	concurrent.Async(func() { trader.trade(client, user) })
	concurrent.Async(func() { trader.switchOnOff(user) })
	concurrent.Async(func() { trader.configure(user) })

	// signal successful start of processor
	user.Send(api.Index(trader.account),
		api.NewMessage(processor.Audit(trader.compoundKey(ProcessorName), "started processor")).
			AddLine(createConfigMessage(trader)), nil)
	return func(in <-chan *model.TrackedOrder, out chan<- *model.TrackedOrder) {
		defer func() {
			log.Info().
				Str("user", id).
				Str("account", trader.account).
				Str("processor", ProcessorName).
				Msg("closing processor")
			close(out)
		}()

		for order := range in {
			log.Debug().
				Str("user", id).
				Str("account", trader.account).
				Str("order", fmt.Sprintf("%+v", order)).
				Msg("received order command")

			// propagate message to others ...
			out <- order
			log.Debug().
				Str("user", id).
				Str("account", trader.account).
				Msg("order command propagated")

			// check if running ...
			if !trader.running {
				log.Debug().
					Str("user", id).
					Str("account", trader.account).
					Bool("running", trader.running).
					Msg("ignoring signal")
				continue
			}

			// handle the order
			coin := order.Coin
			key := order.Key
			t := order.Type
			v := order.Volume
			trader.initConfig(coin)
			//p, pErr := message.Price()
			var err error
			var close string
			var profit float64
			// ignore the MANUAL-sell signals
			// and the BS-buy signals
			// TODO : formalise this logic as a filter
			if (key.Strategy == "MANUAL" && t == model.Sell) ||
				(key.Strategy == "BS" && t == model.Buy) {
				rErr := registry.Add(storage.K{
					Pair:  string(order.Coin),
					Label: fmt.Sprintf("%v_%s", key, ignoredSuffix),
				}, Order{
					Order: *order,
				})
				log.Debug().Err(rErr).
					Str("user", id).
					Str("account", trader.account).
					Str("mode", key.Strategy).
					Msg("error saving ignored signal")
				continue
			}

			// check the positions ...
			position, ok, positions := trader.Check(key, coin)
			if ok {
				// if we had a position already ...
				if position.Type == t {
					// but .. we dont want to extend the current one ...
					log.Debug().
						Str("user", id).
						Str("account", trader.account).
						Str("position", fmt.Sprintf("%+v", position)).
						Msg("ignoring signal")
					continue
				}
				// we need to close the position
				close = position.OrderID
				t = position.Type.Inv()
				v = position.Volume
				log.Debug().
					Str("user", id).
					Str("account", trader.account).
					Str("order", fmt.Sprintf("%+v", order)).
					Str("position", fmt.Sprintf("%+v", position)).
					Str("type", t.String()).
					Float64("volume", v).
					Msg("closing position")
			} else if len(positions) > 0 {
				var ignore bool
				for _, p := range positions {
					if p.Type != t {
						// if it will be an opposite opening to the current position,
						// it will act as a close, and it will break our metrics ...
						ignore = true
					}
				}
				if ignore {
					log.Debug().
						Str("user", id).
						Str("account", trader.account).
						Str("positions", fmt.Sprintf("%+v", positions)).
						Str("order", fmt.Sprintf("%+v", order)).
						Msg("ignoring signal")
					continue
				}
			}
			order.RefID = close
			order, _, err = client.OpenOrder(order)
			if order.RefID != "" {
				// TODO : parse the time from the signal message
				_, profit = position.Value(model.NewPrice(order.Price, time.Now()))
			}
			if err == nil {
				regErr := registry.Add(storage.K{
					Pair:  string(order.Coin),
					Label: key.Strategy,
				}, Order{
					Order: *order,
				})
				trackErr := trader.Submit(key, order, close)
				if regErr != nil || trackErr != nil {
					log.Error().
						Str("registry-error", fmt.Sprintf("%v", regErr)).
						Str("tracker-error", fmt.Sprintf("%v", trackErr)).
						Str("account", trader.account).
						Str("order", fmt.Sprintf("%+v", order)).
						Msg("could not save to registry")
				}
			} else {
				// save to the registry to keep track of the messages anyway
				errs := map[string]string{
					"order": err.Error(),
					//"price":  pErr.Error(),
				}
				regErr := registry.Add(storage.K{
					Pair:  string(order.Coin),
					Label: fmt.Sprintf("%v_%s", key.Hash(), errorSuffix),
				}, Order{
					Order:  *order,
					Errors: errs,
				})
				if regErr != nil {
					log.Error().Err(regErr).
						Str("account", trader.account).
						Str("errors", fmt.Sprintf("%+v", errs)).
						Str("order", fmt.Sprintf("%+v", order)).
						Msg("could not save to registry")
				}
			}
			log.Debug().
				Str("account", trader.account).
				Str("type", t.String()).
				Str("ref-id", order.RefID).
				Float64("profit", profit).
				Str("coin", string(coin)).
				Err(err).
				Msg("processed signal")
			// dont spam the user ...
			// TODO : decide when to show the error though
			if err == nil || t == model.Buy || close != "" {
				user.Send(api.Index(trader.account),
					api.NewMessage(processor.Audit(trader.compoundKey(ProcessorName), "processed signal")).
						AddLine(createTypeMessage(coin, t, order.Audit.Volume, order.Volume, order.Price, close, profit)).
						AddLine(createReportMessage(key.Hash(), key.ToString(), err)),
					nil)
			}
		}
	}
}

func createTypeMessage(coin model.Coin, t model.Type, adjustedVolume string, volume, price float64, close string, profit float64) string {
	return fmt.Sprintf("%s %s %s %s (%.4f) at %.4f | %s %.2f%s",
		string(coin),
		emoji.MapOpen(close == ""),
		emoji.MapType(t),
		adjustedVolume,
		volume,
		price,
		emoji.MapToSign(profit),
		profit,
		"%",
	)
}

func createReportMessage(key, detail string, err ...error) string {
	var errs string
	for _, e := range err {
		if e != nil {
			errs = fmt.Sprintf("%s:%s", errs, e.Error())
		}
	}
	return fmt.Sprintf("%s [%s] | %s", key, detail, errs)
}

func createConfigMessage(trader *trader) string {
	parts := []string{
		emoji.MapBool(trader.running),
		fmt.Sprintf("[%d]", trader.minSize),
		fmt.Sprintf("(%d)", len(trader.positions)),
	}
	return strings.Join(parts, " ")
}
