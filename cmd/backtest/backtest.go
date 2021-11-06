package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/drakos74/free-coin/internal/buffer"

	"github.com/drakos74/free-coin/client/history"
	"github.com/drakos74/free-coin/client/kraken"
	localExchange "github.com/drakos74/free-coin/client/local"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	storage "github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/drakos74/free-coin/user/local"
	"github.com/rs/zerolog/log"
)

const (
	port = 6090
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {

	go func() {
		err := server.NewServer("back-test", port).
			AddRoute(server.GET, server.Test, "run", run()).
			AddRoute(server.GET, server.Test, "load", load()).
			Run()
		if err != nil {
			log.Error().Err(err).Msg("could not start server")
		}
	}()

	<-context.Background().Done()

}

func run() server.Handler {
	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {
		request, err := parse(r.URL.Query())
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
		threshold, err := strconv.Atoi(request.Threshold[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

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
			Time:   make([]time.Time, 0),
			Trades: make([]Point, 0),
			Price:  make([]Point, 0),
			Trigger: Trigger{
				Buy:  make([]Point, 0),
				Sell: make([]Point, 0),
			},
		}

		// reduce data size for viewing purposes
		minutes := to.Sub(from).Minutes()
		d := int(minutes / 1000)
		window := buffer.NewHistoryWindow(time.Minute*time.Duration(d), 1)

		tradeTracker := coin.NewStrategy("Trades").
			ForUser(u).
			WithProcessor(func(u api.User, e api.Exchange) api.Processor {
				return processor.Process("Trades", func(trade *model.Trade) error {
					if b, ok := window.Push(trade.Time, trade.Price); ok {
						price := b.Bucket.Values().Stats()[0].Avg()
						func(trade model.Trade, price float64) {
							response.Trades = append(response.Trades, Point{
								X: trade.Time,
								Y: trade.Price,
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
		signals := make([]*stats.Signal, 0)
		for _, m := range u.Messages {
			signal := new(stats.Signal)
			err := json.Unmarshal([]byte(m.Text), signal)
			if err != nil {
				fmt.Printf("err = %+v\n m = %+v\n", err, m.Text)
			} else {
				signals = append(signals, signal)
			}
		}

		for i := 0; i < 8; i++ {
			if threshold > 0 {
				if i != threshold {
					continue
				}
			}
			r := simulate(signals, i)
			pnl := r.portfolio()
			r.PnL = pnl
			fmt.Printf("i = %+v , r = %+v , v = %+v\n", i, r, pnl)
			if r.portfolio() > response.Details[0].Result.portfolio() || response.Details[0].Result.Trades == 0 {
				response.Details[0].Result = r
			}
		}

		for _, signal := range signals {

			if !signal.Filter(response.Details[0].Result.Threshold) {
				continue
			}

			switch signal.Type {
			case model.Buy:
				response.Trigger.Buy = append(response.Trigger.Buy, Point{
					X: signal.Time,
					Y: signal.Price,
				})
			case model.Sell:
				response.Trigger.Sell = append(response.Trigger.Sell, Point{
					X: signal.Time,
					Y: signal.Price,
				})
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
		//		if !signal.Filter(response.Details[0].Result.Threshold) {
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

		exchange.Gather()

		bb, err := json.Marshal(response)
		if err != nil {
			return []byte(err.Error()), http.StatusInternalServerError, nil
		}

		return bb, http.StatusOK, nil
	}
}

func load() server.Handler {

	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {

		upstream := kraken.NewClient(model.BTC).
			Since(cointime.LastXHours(136)).
			Interval(10 * time.Second)
		client := history.New(upstream).WithRegistry(storage.NewEventRegistry("Trades"))

		//process := make(chan api.Signal)
		//source, err := client.Trades(process)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusInternalServerError, nil
		//}

		engine, err := coin.NewEngine(client)
		if err != nil {
			return []byte(err.Error()), http.StatusInternalServerError, nil
		}

		// create a tracking user
		u, err := local.NewUser("user.log")

		c := 0
		startTime := time.Time{}
		endTime := time.Time{}
		// add a final tracker
		tradeTracker := coin.NewStrategy("Trades").
			ForUser(u).
			WithProcessor(func(u api.User, e api.Exchange) api.Processor {
				return processor.Process("Trades", func(trade *model.Trade) error {
					if c == 0 {
						startTime = trade.Time
					}
					if c%1000 == 0 {
						fmt.Printf("%v | c = %+v\n", trade.Time, c)
					}
					c++
					endTime = trade.Time
					return nil
				})
			}).Apply()

		engine.
			//AddProcessor(positionTracker).
			AddProcessor(tradeTracker)

		err = engine.Run()
		//Trades := make([]*model.Trade, 0)
		//for t := range source {
		//	Trades = append(Trades, t)
		//	// unblock source
		//	process <- api.Signal{}
		//}
		//
		//tt, err := json.Marshal(Trades)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusInternalServerError, nil
		//}

		str := fmt.Sprintf("%v - %v | %d", startTime, endTime, c)

		return []byte(str), http.StatusOK, nil
	}
}
