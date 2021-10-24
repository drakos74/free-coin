package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

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

		from, err := time.Parse("2006_01_02T15", request.From[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}
		to, err := time.Parse("2006_01_02T15", request.To[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}
		client := history.New(nil).Reader(&history.Request{
			Coin: model.Coin(request.Coin[0]),
			From: from,
			To:   to,
		}).WithRegistry(storage.NewEventRegistry("trades"))

		//process := make(chan api.Signal)
		//source, err := client.Trades(process)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusInternalServerError, nil
		//}

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
			WithProcessor(stats.Processor(api.DrCoin, shard, map[model.Coin]map[time.Duration]stats.Config{
				model.BTC: {
					//time.Minute * 2: stats.New("2-min", time.Minute*2).Add(4, 1).Notify().Build(),
					time.Minute * 5: stats.New("5-min", time.Minute*5).Add(5, 1).Notify().Build(),
					//time.Minute * 15: stats.New("15-min", time.Minute*15).Add(2, 1).Notify().Build(),
				},
			})).Apply()
		//positionTracker := coin.NewStrategy(stats.Name).
		//	WithProcessor(position.Processor(api.DrCoin)).
		//	Apply()

		// add a final tracker
		tradeTracker := coin.NewStrategy("trades").
			ForUser(u).
			WithProcessor(func(u api.User, e api.Exchange) api.Processor {
				return processor.Process("trades", func(trade *model.Trade) error {
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
		//trades := make([]*model.Trade, 0)
		//for t := range source {
		//	trades = append(trades, t)
		//	// unblock source
		//	process <- api.Signal{}
		//}
		//
		//tt, err := json.Marshal(trades)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusInternalServerError, nil
		//}

		builder := new(strings.Builder)
		for _, m := range u.Messages {
			builder.WriteString(fmt.Sprintf("%v = %+v\n", m.Time, m.Text))
		}

		exchange.Gather()

		return []byte(builder.String()), http.StatusOK, nil
	}
}

func load() server.Handler {

	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {

		upstream := kraken.NewClient(model.BTC).
			Since(cointime.LastXHours(14 * 24)).
			Interval(10 * time.Second)
		client := history.New(upstream).WithRegistry(storage.NewEventRegistry("trades"))

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
		tradeTracker := coin.NewStrategy("trades").
			ForUser(u).
			WithProcessor(func(u api.User, e api.Exchange) api.Processor {
				return processor.Process("trades", func(trade *model.Trade) error {
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
		//trades := make([]*model.Trade, 0)
		//for t := range source {
		//	trades = append(trades, t)
		//	// unblock source
		//	process <- api.Signal{}
		//}
		//
		//tt, err := json.Marshal(trades)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusInternalServerError, nil
		//}

		str := fmt.Sprintf("%v - %v | %d", startTime, endTime, c)

		return []byte(str), http.StatusOK, nil
	}
}
