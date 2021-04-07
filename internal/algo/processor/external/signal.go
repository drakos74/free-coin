package external

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const (
	ProcessorName = "coin-processor"
	OnOffSwitch   = "coin-processor-coin-on-off"
	port          = 8080
	grafanaPort   = 6124
)

func (t *tracker) compoundKey(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, t.user)
}

func (t *tracker) switchOnOff(user api.User) {
	for command := range user.Listen(t.compoundKey(OnOffSwitch), "?r") {

		if t.user != "" && command.User != t.user {
			continue
		}

		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?p"),
			api.OneOf(&action, "start", "stop"),
		)

		if err != nil {
			api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "error")).ReplyTo(command.ID), err)
			continue
		}

		switch action {
		case "start":
			t.running = true
		case "stop":
			t.running = false
		}
		api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), fmt.Sprintf("%s...ed", action))).ReplyTo(command.ID), nil)

	}
}

func (t *tracker) trackUserActions(client api.Exchange, user api.User) {

	for command := range user.Listen(t.compoundKey(ProcessorName), "?p") {

		if t.user != "" && t.user != command.User {
			continue
		}

		ctx := context.Background()

		errMsg := ""
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?p"),
		)

		if err != nil {
			api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "error")).ReplyTo(command.ID), err)
			continue
		}

		keys, positions, prices := t.getAll(ctx)

		// get account balance first to double check ...
		bb, err := client.Balance(ctx, prices)
		if err != nil {
			errMsg = err.Error()
		}

		freeCoin := model.Coin("free")
		// add the total coin
		total := model.Balance{
			Coin: freeCoin,
		}

		sort.Strings(keys)
		now := time.Now()

		report := api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "positions"))
		if len(positions) == 0 {
			report.AddLine("no open positions")
		}
		for i, k := range keys {
			pos := positions[k]

			since := now.Sub(pos.OpenTime)
			net, profit := pos.Value()
			configMsg := fmt.Sprintf("[ %s ] [ %.0fh ]", k, math.Round(since.Hours()))
			msg := fmt.Sprintf("[%d] %s %.2f%s (%.2f%s) <- %s | %f [%f]",
				i+1,
				emoji.MapToSign(net),
				profit,
				"%",
				pos.OpenPrice,
				emoji.Money,
				emoji.MapType(pos.Type),
				pos.Volume,
				bb[pos.Coin].Volume,
			)

			if balance, ok := bb[pos.Coin]; ok {
				balance.Volume -= pos.Volume
				bb[pos.Coin] = balance
			}

			total.Locked += pos.OpenPrice * pos.Volume
			total.Volume += pos.CurrentPrice * pos.Volume

			// TODO : send a trigger for each Position to give access to adjust it
			//trigger := &api.Trigger{
			//	ID:  pos.ID,
			//	Key: positionKey,
			//}
			report = report.AddLine(msg).AddLine(configMsg).AddLine("************")
			if errMsg != "" {
				report = report.AddLine(fmt.Sprintf("balance:error:%s", errMsg))
			}
		}
		// send all positions report ... to avoid spamming the chat
		user.Send(api.Index(command.User), report, nil)

		balanceReport := api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "balance"))
		for coin, balance := range bb {
			if math.Abs(balance.Volume) > 0.000000001 {
				balanceReport = balanceReport.AddLine(fmt.Sprintf("%s %f -> %f%s",
					string(coin),
					balance.Volume,
					balance.Volume*balance.Price,
					emoji.Money))
			}
		}
		// print also the total ...
		v := (total.Volume - total.Locked) / total.Locked
		balanceReport.AddLine(fmt.Sprintf("%s(%.2f%s) %f%s -> %f%s",
			emoji.MapValue(10*v/2),
			100*v,
			"%",
			total.Locked,
			emoji.Money,
			total.Volume,
			emoji.Money))
		user.Send(api.Index(command.User), balanceReport, nil)
	}
}

type MessageSignal struct {
	Source chan Message
	Output chan Message
}

// TODO : use in a unified channel for all tracked currencies ...
// TODO : this needs to be a combined client pushing many coins to the trade source ...
func Signal(id string, shard storage.Shard, registry storage.Registry, client api.Exchange, user api.User, signal MessageSignal, configs map[model.Coin]map[time.Duration]processor.Config) api.Processor {

	if signal.Source == nil {
		signal.Source = make(chan Message)
		go server.NewServer("trade-view", port).
			AddRoute(server.GET, server.Api, "test-get", handle(user, nil)).
			AddRoute(server.POST, server.Api, "test-post", handle(user, signal.Source)).
			Run()

		grafana := metrics.NewServer("grafana", grafanaPort)
		addTargets("", client, grafana, registry)

		// run the grafana server
		grafana.Run()
	}

	// init tracker related actions
	tracker, err := newTracker(id, client, shard)
	go tracker.trackUserActions(client, user)

	if err != nil {
		log.Error().Err(err).Str("user", tracker.user).Str("processor", ProcessorName).Msg("could not start processor")
		return func(in <-chan *model.Trade, out chan<- *model.Trade) {
			for t := range in {
				out <- t
			}
		}
	}

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("user", tracker.user).Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for {
			select {
			case message := <-signal.Source:
				// propagate message to others ...
				signal.Output <- message
				if !tracker.running {
					// we are in stopped state ...
					log.Debug().
						Str("user", tracker.user).
						Str("message", fmt.Sprintf("%+v", message)).
						Msg("ignoring signal")
					continue
				}
				coin := model.Coin(message.Data.Ticker)
				key := message.Key()
				t, tErr := message.Type()
				v, vErr := message.Volume()
				//p, pErr := message.Price()
				var err error
				var close string
				var order model.TrackedOrder
				if tErr == nil && vErr == nil {
					// check the positions ...
					position, ok, positions := tracker.check(key, coin)
					if ok {
						// if we had a position already ...
						if position.Type == t {
							// but .. we dont want to extend the current one ...
							log.Debug().
								Str("user", tracker.user).
								Str("position", fmt.Sprintf("%+v", position)).
								Msg("ignoring signal")
							//user.Send(api.External,
							//	api.NewMessage("ignoring signal").
							//		AddLine(createTypeMessage(coin, t, v, p, false)).
							//		AddLine(createReportMessage(key, fmt.Errorf("%s:%v:%v", emoji.MapType(position.Type), position.Volume, position.OpenPrice))),
							//	nil)
							continue
						}
						// we need to close the position
						close = position.OrderID
						t = position.Type.Inv()
						v = position.Volume
						log.Debug().
							Str("user", tracker.user).
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
								Str("user", tracker.user).
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
					if err == nil {
						regErr := registry.Add(storage.K{
							Pair:  message.Data.Ticker,
							Label: message.Key(),
						}, Order{
							Message: message,
							Order:   order,
						})
						trackErr := tracker.add(key, order, close)
						if regErr != nil || trackErr != nil {
							log.Error().Err(regErr).Err(trackErr).
								Str("user", tracker.user).
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
								Str("user", tracker.user).
								Str("errors", fmt.Sprintf("%+v", errs)).
								Str("order", fmt.Sprintf("%+v", order)).
								Str("message", fmt.Sprintf("%+v", message)).
								Msg("could not save to registry")
						}
					}
				} else {
					log.Warn().
						Str("user", tracker.user).
						Str("message", fmt.Sprintf("%+v", message)).
						Msg("could not parse message")
				}
				log.Info().
					Str("user", tracker.user).
					Str("type", t.String()).
					Str("coin", string(coin)).
					Err(tErr).Err(vErr).Err(err).
					Msg("processed signal")
				user.Send(api.Index(tracker.user),
					api.NewMessage(processor.Audit(tracker.compoundKey(ProcessorName), "processed signal")).
						AddLine(createTypeMessage(coin, t, order.Volume, order.Price, close)).
						AddLine(createReportMessage(key, tErr, vErr, err)),
					nil)
			case trade := <-in:
				out <- trade
			}
		}
	}
}

func createTypeMessage(coin model.Coin, t model.Type, volume, price float64, close string) string {
	return fmt.Sprintf("%s %s %s %.2f at %.2f", string(coin), emoji.MapBool(close == ""), emoji.MapType(t), volume, price)
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
