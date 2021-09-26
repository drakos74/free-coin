package main

import (
	"log"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/algo/processor/signal"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/user/local"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	// main engine trade input ...
	client := kraken.NewClient(model.BTC, model.ETH).
		//Since(cointime.LastXHours(24)).
		Interval(5 * time.Second)
	engine, err := coin.NewEngine(client)
	if err != nil {
		log.Fatalf("error creating engine: %s", err.Error())
	}

	// position tracker for kraken
	exchange := kraken.NewExchange(account.Drakos)
	u, err := local.NewUser("")
	if err != nil {
		log.Fatalf("error creating user: %s", err.Error())
	}
	positionTracker := coin.NewStrategy("position-tracker").
		ForExchange(exchange).
		ForUser(u).
		WithProcessor(position.Processor(api.DrCoin)).Apply()
	engine.AddProcessor(positionTracker)

	// signal processor from tradeview
	signalProcessor := coin.NewStrategy("signal-processor").
		ForExchange(exchange).
		ForUser(u).
		WithProcessor(signal.New()).Apply()
	engine.AddProcessor(signalProcessor)

	err = engine.Run()
	if err != nil {
		log.Fatalf("error running engine: %s", err.Error())
	}
}
