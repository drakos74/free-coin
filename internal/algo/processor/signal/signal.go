package signal

import (
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const (
	ProcessorName = "signal"
	port          = 8080
	grafanaPort   = 6124
)

type MessageSignal struct {
	Source chan Message
	Output chan Message
}

func Receiver(id string, shard storage.Shard, eventRegistry storage.EventRegistry, client api.Exchange, user api.User, signal MessageSignal, settings map[model.Coin]map[time.Duration]Settings) api.Processor {

	registry, err := eventRegistry(id)
	if err != nil {
		log.Error().Err(err).Msg("could not create registry")
		return processor.Void(id)
	}

	// init trader related actions
	trader, err := newTrader(id, client, shard, settings)
	if err != nil {
		log.Error().Str("user", id).Err(err).Msg("could not start tracker")
	}
	go trader.trackUserActions(client, user)
	go trader.trade(client, user)
	go trader.switchOnOff(user)
	go trader.configure(user)

	if err != nil {
		log.Error().Err(err).Str("account", trader.account).Str("processor", ProcessorName).Msg("could not start processor")
		user.Send(api.Index(trader.account),
			api.NewMessage(processor.Error(trader.compoundKey(ProcessorName), err)), nil)
		return func(in <-chan *model.Trade, out chan<- *model.Trade) {
			for t := range in {
				out <- t
			}
		}
	}

	// signal successful start of processor
	user.Send(api.Index(trader.account),
		api.NewMessage(processor.Audit(trader.compoundKey(ProcessorName), "started processor")).
			AddLine(createConfigMessage(trader)), nil)
	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("account", trader.account).Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for {
			select {
			case message := <-signal.Source:
				log.Debug().
					Str("user", trader.account).
					Str("message", fmt.Sprintf("%+v", message)).
					Msg("received message")
				// propagate message to others ...
				signal.Output <- message
				log.Debug().
					Str("user", trader.account).
					Msg("message propagated")
				if !trader.running {
					// we are in stopped state ...
					log.Debug().
						Str("account", trader.account).
						Str("message", fmt.Sprintf("%+v", message)).
						Msg("ignoring signal")
					continue
				}
				coin := model.Coin(message.Data.Ticker)
				key := message.Key()
				t, tErr := message.Type()
				v, b, vErr := message.Volume(trader.minSize)
				trader.parseConfig(coin, b)
				//p, pErr := message.Price()
				var err error
				var close string
				var order model.TrackedOrder
				var profit float64
				if tErr == nil && vErr == nil {
					// ignore the MANUAL-sell signals
					// and the BS-buy signals
					if (message.Config.Mode == "MANUAL" && t == model.Sell) ||
						(message.Config.Mode == "BS" && t == model.Buy) {
						rErr := registry.Add(storage.K{
							Pair:  fmt.Sprintf("%s_%s_ignored", message.Data.Ticker, message.Config.Mode),
							Label: message.Detail(),
						}, Order{
							Message: message,
						})
						log.Debug().Str("mode", message.Config.Mode).Err(rErr).Msg("error saving ignored signal")
						continue
					}

					// check the positions ...
					position, ok, positions := trader.check(key, coin)
					if ok {
						// if we had a position already ...
						if position.Type == t {
							// but .. we dont want to extend the current one ...
							log.Debug().
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
							Str("account", trader.account).
							Str("message", fmt.Sprintf("%+v", message)).
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
								Str("account", trader.account).
								Str("positions", fmt.Sprintf("%+v", positions)).
								Msg("ignoring conflicting signal")
							continue
						}
					}
					order = model.NewOrder(coin).
						Market().
						WithType(t).
						WithVolume(v).
						CreateTracked(model.Key{
							Coin:     coin,
							Duration: message.Duration(),
							Strategy: message.Key(),
						}, message.Time())
					order.RefID = close
					order, _, err = client.OpenOrder(order)
					if order.RefID != "" {
						// TODO : parse the time from the signal message
						_, profit = position.Value(model.NewPrice(order.Price, time.Now()))
					}
					if err == nil {
						regErr := registry.Add(storage.K{
							Pair:  message.Data.Ticker,
							Label: message.Key(),
						}, Order{
							Message: message,
							Order:   order,
						})
						trackErr := trader.add(key, order, close)
						if regErr != nil || trackErr != nil {
							log.Error().
								Str("registry-error", fmt.Sprintf("%v", regErr)).
								Str("tracker-error", fmt.Sprintf("%v", trackErr)).
								Str("account", trader.account).
								Str("order", fmt.Sprintf("%+v", order)).
								Str("message", fmt.Sprintf("%+v", message)).
								Msg("could not save to registry")
						}
					} else {
						tError := ""
						if tErr != nil {
							tError = tErr.Error()
						}
						vError := ""
						if vErr != nil {
							vError = vErr.Error()
						}
						// save to the registry to keep track of the messages anyway
						errs := map[string]string{
							"order":  err.Error(),
							"type":   tError,
							"volume": vError,
							//"price":  pErr.Error(),
						}
						regErr := registry.Add(storage.K{
							Pair:  message.Data.Ticker,
							Label: fmt.Sprintf("%s_%s", message.Key(), "error"),
						}, Order{
							Message: message,
							Order:   order,
							Errors:  errs,
						})
						if regErr != nil {
							log.Error().Err(regErr).
								Str("account", trader.account).
								Str("errors", fmt.Sprintf("%+v", errs)).
								Str("order", fmt.Sprintf("%+v", order)).
								Str("message", fmt.Sprintf("%+v", message)).
								Msg("could not save to registry")
						}
					}
				} else {
					log.Warn().
						Str("account", trader.account).
						Str("message", fmt.Sprintf("%+v", message)).
						Msg("could not parse message")
				}
				log.Debug().
					Str("account", trader.account).
					Str("type", t.String()).
					Str("ref-id", order.RefID).
					Float64("profit", profit).
					Str("coin", string(coin)).
					Err(tErr).Err(vErr).Err(err).
					Msg("processed signal")
				// dont spam the user ...
				// TODO : decide when to show the error though
				if err == nil || t == model.Buy || close != "" {
					user.Send(api.Index(trader.account),
						api.NewMessage(processor.Audit(trader.compoundKey(ProcessorName), "processed signal")).
							AddLine(createTypeMessage(coin, t, order.Volume, order.Price, close, profit)).
							AddLine(createReportMessage(key, message.Detail(), tErr, vErr, err)),
						nil)
				}
			case trade := <-in:
				out <- trade
			}
		}
	}
}

func createTypeMessage(coin model.Coin, t model.Type, volume, price float64, close string, profit float64) string {
	return fmt.Sprintf("%s %s %s %.4f at %.4f | %s %.2f%s",
		string(coin),
		emoji.MapOpen(close == ""),
		emoji.MapType(t),
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
	return fmt.Sprintf("%s-(%s)-[%s]", key, detail, errs)
}

func createConfigMessage(trader *trader) string {
	parts := []string{
		emoji.MapBool(trader.running),
		fmt.Sprintf("[%d]", trader.minSize),
		fmt.Sprintf("(%d)", len(trader.positions)),
	}
	return strings.Join(parts, " ")
}
