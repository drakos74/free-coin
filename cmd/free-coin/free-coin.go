package main

import (
	"context"
	"log"
	"time"

	cointime "github.com/drakos74/free-coin/internal/time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/algo/processor/ml"
	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	json_storage "github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/user/telegram"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	config := configML()

	cc := make([]model.Coin, 0)
	coins := make(map[model.Coin]bool)
	for k, _ := range config.Segments {
		if !coins[k.Coin] {
			cc = append(cc, k.Coin)
			coins[k.Coin] = true
		}
	}

	// main engine trade input ...
	client := kraken.NewClient(cc...).
		Since(cointime.LastXHours(48)).
		Interval(2 * time.Second)
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
	//positionTracker := coin.NewStrategy("position-tracker").
	//	ForExchange(exchange).
	//	ForUser(u).
	//	WithProcessor(position.Processor(api.FreeCoin)).Apply()
	//engine.AddProcessor(positionTracker)

	shard := json_storage.BlobShard("ml")
	registry := json_storage.EventRegistry("ml-event-registry")
	processor := mlProcessor(u, exchange, shard, registry)
	engine.AddProcessor(processor)

	//signal processor from tradeview
	//signalProcessor := coin.NewStrategy("signal-processor").
	//	ForExchange(exchange).
	//	ForUser(u).
	//	WithProcessor(signal.New()).Apply()
	//engine.AddProcessor(signalProcessor)

	go u.Run(context.Background())
	err = engine.Run()
	if err != nil {
		log.Fatalf("error running engine: %s", err.Error())
	}
}

func mlProcessor(u api.User, e api.Exchange, shard storage.Shard, registry storage.EventRegistry) api.Processor {
	return coin.NewStrategy(ml.Name).
		ForUser(u).
		ForExchange(e).
		WithProcessor(ml.Processor(api.FreeCoin, shard, registry, nil, configML())).
		Apply()
}

func configML() ml.Config {
	cfg := map[model.Key]ml.Segments{
		model.Key{
			Coin:     model.BTC,
			Duration: 15 * time.Minute,
			Strategy: "btc",
		}: {
			Stats: ml.Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.15,
			},
			Model: ml.Model{
				BufferSize:         3,
				PrecisionThreshold: 0.5,
				ModelSize:          10,
				Features:           4,
			},
			Trader: ml.Trader{
				BufferTime:     0,
				PriceThreshold: 0,
				Weight:         0,
			},
		},
		model.Key{
			Coin:     model.DOT,
			Duration: 15 * time.Minute,
			Strategy: "dot",
		}: {
			Stats: ml.Stats{
				LookBack:  3,
				LookAhead: 1,
				Gap:       0.15,
			},
			Model: ml.Model{
				BufferSize:         3,
				PrecisionThreshold: 0.5,
				ModelSize:          10,
				Features:           4,
			},
			Trader: ml.Trader{
				BufferTime:     0,
				PriceThreshold: 0,
				Weight:         0,
			},
		},
		model.Key{
			Coin:     model.ETH,
			Duration: 15 * time.Minute,
			Strategy: "eth",
		}: {
			Stats: ml.Stats{
				LookBack:  6,
				LookAhead: 1,
				Gap:       0.15,
			},
			Model: ml.Model{
				BufferSize:         3,
				PrecisionThreshold: 0.5,
				ModelSize:          10,
				Features:           4,
			},
			Trader: ml.Trader{
				BufferTime:     0,
				PriceThreshold: 0,
				Weight:         0,
			},
		},
		model.Key{
			Coin:     model.LINK,
			Duration: 15 * time.Minute,
			Strategy: "link",
		}: {
			Stats: ml.Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.15,
			},
			Model: ml.Model{
				BufferSize:         5,
				PrecisionThreshold: 0.5,
				ModelSize:          10,
				Features:           4,
			},
			Trader: ml.Trader{
				BufferTime:     0,
				PriceThreshold: 0,
				Weight:         0,
			},
		},
		model.Key{
			Coin:     model.SOL,
			Duration: 15 * time.Minute,
			Strategy: "sol",
		}: {
			Stats: ml.Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.15,
			},
			Model: ml.Model{
				BufferSize:         5,
				PrecisionThreshold: 0.5,
				ModelSize:          10,
				Features:           4,
			},
			Trader: ml.Trader{
				BufferTime:     0,
				PriceThreshold: 0,
				Weight:         0,
			},
		},
		model.Key{
			Coin:     model.FLOW,
			Duration: 15 * time.Minute,
			Strategy: "flow",
		}: {
			Stats: ml.Stats{
				LookBack:  6,
				LookAhead: 1,
				Gap:       0.15,
			},
			Model: ml.Model{
				BufferSize:         3,
				PrecisionThreshold: 0.5,
				ModelSize:          10,
				Features:           4,
			},
			Trader: ml.Trader{
				BufferTime:     0,
				PriceThreshold: 0,
				Weight:         0,
			},
		},
		model.Key{
			Coin:     model.MATIC,
			Duration: 15 * time.Minute,
			Strategy: "matic",
		}: {
			Stats: ml.Stats{
				LookBack:  6,
				LookAhead: 1,
				Gap:       0.15,
			},
			Model: ml.Model{
				BufferSize:         8,
				PrecisionThreshold: 0.5,
				ModelSize:          10,
				Features:           4,
			},
			Trader: ml.Trader{
				BufferTime:     0,
				PriceThreshold: 0,
				Weight:         0,
			},
		},
	}

	return ml.Config{
		Segments: cfg,
		Position: ml.Position{
			OpenValue:  500,
			StopLoss:   0.02,
			TakeProfit: 0.02,
		},
		Debug:     false,
		Benchmark: true,
	}
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
