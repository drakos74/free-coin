package main

import (
	"log"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	storage "github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/user/telegram"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
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
	u, err := telegram.NewBot(api.FreeCoin)
	if err != nil {
		log.Fatalf("error creating user: %s", err.Error())
	}
	positionTracker := coin.NewStrategy("position-tracker").
		ForExchange(exchange).
		ForUser(u).
		WithProcessor(position.Processor(api.FreeCoin)).Apply()
	engine.AddProcessor(positionTracker)

	shard := storage.BlobShard("stats")
	statsTracker := coin.NewStrategy(stats.Name).
		ForUser(u).
		WithProcessor(stats.Processor(api.FreeCoin, shard, map[model.Coin]map[time.Duration]stats.Config{
			model.BTC: {
				//time.Minute * 2: stats.New("2-min", time.Minute*2).Add(4, 1).Notify().Build(),
				time.Minute * 5: stats.New("5-min", time.Minute*5).Add(5, 1).Build(),
				//time.Minute * 15: stats.New("15-min", time.Minute*15).Add(2, 1).Notify().Build(),
			},
			model.ETH: {
				//time.Minute * 2: stats.New("2-min", time.Minute*2).Add(4, 1).Notify().Build(),
				time.Minute * 5: stats.New("5-min", time.Minute*5).Add(5, 1).Build(),
				//time.Minute * 15: stats.New("15-min", time.Minute*15).Add(2, 1).Notify().Build(),
			},
		})).Apply()
	engine.AddProcessor(statsTracker)

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
