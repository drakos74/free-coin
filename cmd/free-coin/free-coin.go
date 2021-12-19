package main

import (
	"log"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/algo/processor/ml"
	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	json_storage "github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/user/telegram"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func main() {
	// main engine trade input ...
	client := kraken.NewClient(model.BTC, model.ETH, model.DOT).
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

	shard := json_storage.BlobShard("ml")

	processor := mlProcessor(u, exchange, shard)
	engine.AddProcessor(processor)

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

func mlProcessor(u api.User, e api.Exchange, shard storage.Shard) api.Processor {
	return coin.NewStrategy(ml.Name).
		ForUser(u).
		ForExchange(e).
		WithProcessor(ml.Processor(api.FreeCoin, shard, nil, configML(""))).Apply()
}

func configML(m string) ml.Config {
	cfg := map[model.Coin]map[time.Duration]ml.Segments{
		model.BTC: {
			//2 * time.Minute: ml.Segments{
			//	LookBack:  19,
			//	LookAhead: 1,
			//	Threshold: 0.5,
			//},
			//5 * time.Minute: ml.Segments{
			//	LookBack:  19,
			//	LookAhead: 1,
			//	Threshold: 0.5,
			//},
			15 * time.Minute: ml.Segments{
				LookBack:  19,
				LookAhead: 1,
				Threshold: 0.75,
				Model:     "true",
			},
			30 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 1,
				Model:     "true",
			},
			60 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 1,
				Model:     "true",
			},
			//120 * time.Minute: ml.Segments{
			//	LookBack:  4,
			//	LookAhead: 1,
			//	Threshold: 1,
			//},
		},
	}

	return cfg
}

func statsProcessor(u api.User, shard storage.Shard) api.Processor {
	return coin.NewStrategy(stats.Name).
		ForUser(u).
		WithProcessor(stats.Processor(api.FreeCoin, shard, map[model.Coin]map[time.Duration]stats.Config{
			model.BTC: {
				//time.Minute * 2: stats.New("2-min", time.Minute*2).Add(4, 1).Notify().Build(),
				time.Minute * 5: stats.New("5-min", time.Minute*5, 4).Add(5, 5).Build(),
				time.Minute * 2: stats.New("2-min", time.Minute*2, 0).Add(5, 5).Build(),

				//time.Minute * 15: stats.New("15-min", time.Minute*15).Add(2, 1).Notify().Build(),
			},
			model.ETH: {
				//time.Minute * 2: stats.New("2-min", time.Minute*2).Add(4, 1).Notify().Build(),
				time.Minute * 5: stats.New("5-min", time.Minute*5, 4).Add(5, 5).Build(),
				time.Minute * 2: stats.New("2-min", time.Minute*2, 0).Add(5, 5).Build(),
				//time.Minute * 15: stats.New("15-min", time.Minute*15).Add(2, 1).Notify().Build(),
			},
			model.DOT: {
				//time.Minute * 2: stats.New("2-min", time.Minute*2).Add(4, 1).Notify().Build(),
				time.Minute * 5: stats.New("5-min", time.Minute*5, 4).Add(5, 5).Build(),
				time.Minute * 2: stats.New("2-min", time.Minute*2, 0).Add(5, 5).Build(),
				//time.Minute * 15: stats.New("15-min", time.Minute*15).Add(2, 1).Notify().Build(),
			},
		})).Apply()
}
