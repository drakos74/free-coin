package external

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	"github.com/rs/zerolog/log"
)

const (
	ProcessorName = "signal"
	port          = 8080
)

// TODO : use in a unified channel for all tracked currencies ...
// TODO : this needs to be a combined client pushing many coins to the trade source ...
func Signal(shard storage.Shard, registry storage.Registry, client api.Exchange, user api.User, configs map[model.Coin]map[time.Duration]processor.Config) api.Processor {

	signal := make(chan Message)

	go server.NewServer("trade-view", port).
		Add(server.Live()).
		AddRoute(server.GET, server.Api, "test-get", handle(user, nil)).
		AddRoute(server.POST, server.Api, "test-post", handle(user, signal)).
		Run()

	tracker, err := newTracker(shard)
	if err != nil {
		log.Error().Err(err).Str("processor", ProcessorName).Msg("could not start processor")
		return func(in <-chan *model.Trade, out chan<- *model.Trade) {
			for t := range in {
				out <- t
			}
		}
	}

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for {
			select {
			case message := <-signal:
				coin := model.Coin(message.Data.Ticker)
				key := message.Key()
				t, tErr := message.Type()
				v, vErr := message.Volume()
				p, pErr := message.Price()
				var err error
				var close bool
				if tErr == nil && vErr == nil {
					// check the positions ...
					position, ok := tracker.check(key)
					if ok {
						// if we had a position already ...
						if position.Type == t {
							// but .. we dont want to extend the current one ...
							log.Info().
								Str("position", fmt.Sprintf("%+v", position)).
								Msg("ignoring signal")
							user.Send(api.External,
								api.NewMessage("ignoring signal").
									AddLine(createTypeMessage(coin, t, v, p, false)).
									AddLine(createReportMessage(key, fmt.Errorf("ignored signal:%v", position))),
								nil)
							continue
						}
						// we need to close the position
						close = true
						t = position.Type.Inv()
						v = position.Volume
					}
					order := model.NewOrder(coin).
						Market().
						WithType(t).
						WithVolume(v).
						CreateTracked(model.Key{
							Coin:     coin,
							Duration: message.Duration(),
							Strategy: message.Key(),
						}, message.Time())
					_, err = client.OpenOrder(order)
					if err == nil {
						regErr := registry.Add(storage.K{
							Pair:  message.Data.Ticker,
							Label: message.Key(),
						}, Order{
							Message: message,
							Order:   order,
						})
						trackErr := tracker.add(key, order.Order, close)
						if regErr != nil || trackErr != nil {
							log.Error().Err(regErr).Err(trackErr).
								Str("order", fmt.Sprintf("%+v", order)).
								Str("message", fmt.Sprintf("%+v", message)).
								Msg("could not save to registry")
						}
					}
				} else {
					log.Warn().
						Str("message", fmt.Sprintf("%+v", message)).
						Msg("could not parse message")
				}
				log.Info().
					Str("type", t.String()).
					Str("coin", string(coin)).
					Err(tErr).Err(vErr).Err(pErr).Err(err).
					Msg("processed signal")
				user.Send(api.External,
					api.NewMessage("processed signal").
						AddLine(createTypeMessage(coin, t, v, p, close)).
						AddLine(createReportMessage(key, tErr, vErr, pErr, err)),
					nil)
			case trade := <-in:
				out <- trade
			}
		}
	}
}

func createTypeMessage(coin model.Coin, t model.Type, volume, price float64, close bool) string {
	action := "open"
	if close {
		action = "close"
	}
	return fmt.Sprintf("%s %s %s %.2f at %.2f", string(coin), action, t.String(), volume, price)
}

func createReportMessage(key string, err ...error) string {
	var errs string
	for _, e := range err {
		if e != nil {
			errs = fmt.Sprintf("%s:%s", errs, e.Error())
		}
	}
	return fmt.Sprintf("%s [%s]", key, errs)
}
