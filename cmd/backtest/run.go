package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	model2 "github.com/drakos74/free-coin/internal/algo/processor/ml/model"

	"github.com/drakos74/free-coin/client/history"
	localExchange "github.com/drakos74/free-coin/client/local"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	storage "github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/user/local"
	"github.com/rs/zerolog/log"
)

func run() server.Handler {
	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {
		request, err := parseQuery(r.URL.Query())
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		if len(request.Interval) == 0 {
			return []byte("interval parameter missing"), http.StatusBadRequest, nil
		}
		duration, err := strconv.Atoi(request.Interval[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		if len(request.Prev) == 0 {
			return []byte("interval parameter missing"), http.StatusBadRequest, nil
		}
		prev, err := strconv.Atoi(request.Prev[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		if len(request.Next) == 0 {
			return []byte("interval parameter missing"), http.StatusBadRequest, nil
		}
		next, err := strconv.Atoi(request.Next[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		if len(request.Threshold) == 0 {
			return []byte("interval parameter missing"), http.StatusBadRequest, nil
		}
		//threshold, err := strconv.Atoi(request.Threshold[0])
		//if err != nil {
		//	return []byte(err.Error()), http.StatusBadRequest, nil
		//}

		requestCoin := model.Coin(request.Coin[0])
		from, err := time.Parse("2006_01_02T15", request.From[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}
		to, err := time.Parse("2006_01_02T15", request.To[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}
		client := history.New(nil).Reader(&history.Request{
			Coin: requestCoin,
			From: from,
			To:   to,
		}).WithRegistry(storage.NewEventRegistry("Trades"))

		engine, err := coin.NewEngine(client)
		if err != nil {
			return []byte(err.Error()), http.StatusInternalServerError, nil
		}

		shard := storage.BlobShard("stats")

		// create a tracking user
		u, err := local.NewUser("user.log")
		exchange := localExchange.NewExchange("exchange.log")
		statsTracker := coin.NewStrategy(stats.Name).
			ForUser(u).
			ForExchange(exchange).
			WithProcessor(stats.Processor(api.FreeCoin, shard, map[model.Coin]map[time.Duration]stats.Config{
				requestCoin: {
					//time.Minute * 2: stats.New("2-min", time.Minute*2).Add(4, 1).Notify().Build(),
					time.Minute * time.Duration(duration): stats.New(fmt.Sprintf("%d-min", duration), time.Minute*time.Duration(duration), -1).Add(prev, next).Notify().Build(),
					//time.Minute * 15: stats.New("15-min", time.Minute*15).Add(2, 1).Notify().Build(),
				},
			})).Apply()
		//positionTracker := coin.NewStrategy(stats.Name).
		//	WithProcessor(position.Processor(api.DrCoin)).
		//	Apply()

		response := Response{
			Details: []Details{
				{
					Coin:     string(requestCoin),
					Duration: duration,
					Prev:     prev,
					Next:     next,
				},
			},
			Time:    make([]time.Time, 0),
			Trades:  make([]Point, 0),
			Price:   make([]Point, 0),
			Trigger: make(map[string]Trigger),
		}

		// reduce data size for viewing purposes
		minutes := to.Sub(from).Minutes()
		d := int(minutes / 1000)
		window := buffer.NewHistoryWindow(time.Minute*time.Duration(d), 1)

		tradeTracker := coin.NewStrategy("Trades").
			ForUser(u).
			WithProcessor(func(u api.User, e api.Exchange) api.Processor {
				return processor.Process("Trades", func(trade *model.TradeSignal) error {
					if b, ok := window.Push(trade.Time, trade.Tick.Price); ok {
						price := b.Bucket.Values().Stats()[0].Avg()
						func(trade model.TradeSignal, price float64) {
							response.Trades = append(response.Trades, Point{
								X: trade.Time,
								Y: trade.Tick.Price,
							})
							response.Time = append(response.Time, trade.Time)
							response.Price = append(response.Price, Point{
								X: trade.Time,
								Y: price,
							})
						}(*trade, price)
					}
					exchange.Process(trade)
					//u.Send(api.DrCoin, api.NewMessage(fmt.Sprintf("%v:%s:%4.f", trade.Time, trade.Coin, trade.Price)), nil)
					return nil
				})
			}).Apply()

		engine.
			//AddProcessor(positionTracker).
			AddProcessor(statsTracker).
			AddProcessor(tradeTracker)

		err = engine.Run()

		// gather all signals for different scenarios
		log.Info().Int("count", len(u.Messages)).Msg("messages")
		signals := make([]*model2.Signal, 0)
		for _, m := range u.Messages {
			signal := new(model2.Signal)
			err := json.Unmarshal([]byte(m.Text), signal)
			if err != nil {
				fmt.Printf("err = %+v\n m = %+v\n", err, m.Text)
			} else {
				signals = append(signals, signal)
			}
		}

		for _, signal := range signals {

			if !signal.Filter(response.Details[0].Result.Threshold) {
				continue
			}

			// TODO : retrieve the key back from the signal
			key := model.Key{
				Coin:     signal.Key.Coin,
				Duration: signal.Key.Duration,
			}
			if signal.Key.Coin != "" {
				key = signal.Key
			}

			if _, ok := response.Trigger[key.ToString()]; !ok {
				response.Trigger[key.ToString()] = Trigger{
					Buy:  make([]Point, 0),
					Sell: make([]Point, 0),
				}
			}

			var buy []Point
			var sell []Point

			switch signal.Type {
			case model.Buy:
				buy = append(response.Trigger[key.ToString()].Buy, Point{
					X: signal.Time,
					Y: signal.Price,
				})
			case model.Sell:
				sell = append(response.Trigger[key.ToString()].Sell, Point{
					X: signal.Time,
					Y: signal.Price,
				})
			}

			response.Trigger[key.ToString()] = Trigger{
				Buy:  buy,
				Sell: sell,
			}

		}

		//for i := 0; i < len(response.Time); i++ {
		//	t := response.Time[i]
		//	found := []bool{false, false}
		//	for {
		//		if j >= len(signals) {
		//			break
		//		}
		//		signal := signals[j]
		//
		//		if !signal.Filter(response.Details[0].Result.Gap) {
		//			j++
		//			continue
		//		}
		//
		//		if signal.Time.Before(t) {
		//
		//			if signal.Type == model.Buy {
		//				response.Trigger.Buy[i] = signal.Price
		//				if found[0] {
		//					fmt.Printf("overwriting buy signal = %+v\n", response.Trigger.Buy[i])
		//				}
		//				found[0] = true
		//			} else if signal.Type == model.Sell {
		//				response.Trigger.Sell[i] = signal.Price
		//				if found[1] {
		//					fmt.Printf("overwriting sell signal = %+v\n", response.Trigger.Sell[i])
		//				}
		//				found[1] = true
		//			}
		//
		//			j++
		//		} else {
		//			break
		//		}
		//	}
		//	if !found[0] {
		//		if i > 0 {
		//			response.Trigger.Buy[i] = response.Trigger.Buy[i-1]
		//		} else {
		//			response.Trigger.Buy[i] = response.Price[i]
		//		}
		//	}
		//	if !found[1] {
		//		if i > 0 {
		//			response.Trigger.Sell[i] = response.Trigger.Sell[i-1]
		//		} else {
		//			response.Trigger.Sell[i] = response.Price[i]
		//		}
		//	}
		//}

		exchange.Gather(true)

		bb, err := json.Marshal(response)
		if err != nil {
			return []byte(err.Error()), http.StatusInternalServerError, nil
		}

		return bb, http.StatusOK, nil
	}
}
