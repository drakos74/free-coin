package main

import (
	"log"
	"time"

	"github.com/drakos74/free-coin/internal/storage/file/json"

	"github.com/drakos74/free-coin/client/history"

	"github.com/drakos74/free-coin/user/telegram"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func main() {
	// main engine trade input ...
	client := history.New(
		kraken.NewClient(model.BTC, model.ETH).
			//Since(cointime.LastXHours(24)).
			Interval(5 * time.Second),
	).WithRegistry(json.NewEventRegistry("trades"))
	engine, err := coin.NewEngine(client)
	if err != nil {
		log.Fatalf("error creating engine: %s", err.Error())
	}

	// position tracker for kraken
	exchange := kraken.NewExchange(account.Drakos)
	u, err := telegram.NewBot(api.FreeCoin)
	if err != nil {
		log.Fatalf("error creating user: %s", err.Error())
	}
	positionTracker := coin.NewStrategy("position-tracker").
		ForExchange(exchange).
		ForUser(u).
		WithProcessor(position.Processor(api.FreeCoin)).Apply()
	engine.AddProcessor(positionTracker)

	//signal processor from tradeview
	//signalProcessor := coin.NewStrategy("signal-processor").
	//	ForExchange(exchange).
	//	ForUser(u).
	//	WithProcessor(signal.New()).Apply()
	//engine.AddProcessor(signalProcessor)

	err = engine.Run()
	if err != nil {
		log.Fatalf("error running engine: %s", err.Error())
	}
}
