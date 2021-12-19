package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/drakos74/free-coin/client/history"
	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	storage "github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/drakos74/free-coin/user/local"
	"github.com/rs/zerolog/log"
)

func load() server.Handler {

	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {

		request, err := parseQuery(r.URL.Query())
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		params, err := NewRequest(request)
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		upstream := kraken.NewClient(params.Coin).
			Since(cointime.ToNano(params.From)).
			Interval(10 * time.Second).
			Stop(func(trade *model.Trade, numberOfTrades int) bool {
				if trade.Time.After(params.To) {
					return true
				}
				return false
			})
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

func hist() server.Handler {

	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {

		request, err := parseQuery(r.URL.Query())
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		params, err := NewRequest(request)
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		client := history.New(nil).WithRegistry(storage.NewEventRegistry("Trades"))

		ranges := client.Ranges(params.Coin, params.From, params.To)

		str, err := json.Marshal(ranges)
		if err != nil {
			log.Error().Err(err).Msg("could not encode response")
		}

		return str, http.StatusOK, nil
	}
}
