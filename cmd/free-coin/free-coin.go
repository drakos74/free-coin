package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/algo/processor/ml"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/algo/processor/ml/net"
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
		//Since(cointime.LastXHours(48)).
		//Interval(2 * time.Second).
		Live(true)
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
		WithProcessor(ml.Processor(api.FreeCoin, shard, registry,
			net.MultiNetworkConstructor(
				net.ConstructRandomForest(false),
				net.ConstructRandomForest(false),
				net.ConstructRandomForest(false),
			),
			configML())).
		Apply()
}

func StatsConfig(gap float64) mlmodel.Stats {
	return mlmodel.Stats{
		LookBack:  3,
		LookAhead: 1,
		Gap:       gap,
	}
}

func ModelConfig(precision float64) mlmodel.Model {
	return mlmodel.Model{
		BufferSize:         42,
		PrecisionThreshold: precision,
		ModelSize:          120,
		Features:           7,
	}
}

func TraderConfig() mlmodel.Trader {
	return mlmodel.Trader{
		BufferTime:     0,
		PriceThreshold: 0,
		Weight:         1,
	}
}

func ConfigKey(coin model.Coin, d int) model.Key {
	return model.Key{
		Coin:     coin,
		Duration: time.Duration(d) * time.Minute,
		Strategy: fmt.Sprintf("%s_%d", string(coin), d),
	}
}

func configML() *mlmodel.Config {
	cfg := map[model.Key]mlmodel.Segments{
		ConfigKey(model.BTC, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.BTC, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.DOT, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.DOT, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.ETH, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.ETH, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.LINK, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.LINK, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.SOL, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.SOL, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.FLOW, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.FLOW, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.MATIC, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.MATIC, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.AAVE, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.AAVE, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.KSM, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.KSM, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
		ConfigKey(model.KAVA, 15): {
			Stats:  StatsConfig(0.1),
			Model:  ModelConfig(0.55),
			Trader: TraderConfig(),
		},
		ConfigKey(model.KAVA, 6): {
			Stats:  StatsConfig(0.05),
			Model:  ModelConfig(0.6),
			Trader: TraderConfig(),
		},
	}

	return &mlmodel.Config{
		Segments: cfg,
		Position: mlmodel.Position{
			OpenValue:  500,
			StopLoss:   0.01,
			TakeProfit: 0.005,
			TrackingConfig: []*model.TrackingConfig{{
				Duration:  20 * time.Second,
				Samples:   3,
				Threshold: []float64{0.0001, 0.000015},
			}},
		},
		Option: mlmodel.Option{
			Debug:     true,
			Benchmark: true,
		},
		Buffer: mlmodel.Buffer{
			Interval: time.Minute,
		},
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
