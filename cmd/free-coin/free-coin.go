package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor/ml/net"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/algo/processor/ml"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
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
		WithProcessor(ml.Processor(api.FreeCoin, shard, registry, configML(),
			//net.ConstructRandomForestNetwork(false),
			//net.ConstructRandomForestNetwork(false),
			net.ConstructRandomForest(false),
			net.ConstructRandomForest(false),
			//net.ConstructHMM(),
		)).
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
		Features:           8,
	}
}

func TraderConfig(live bool) mlmodel.Trader {
	return mlmodel.Trader{
		BufferTime:     0,
		PriceThreshold: 0,
		Weight:         1,
		Live:           live,
	}
}

func ConfigKey(coin model.Coin, d int) model.Key {
	return model.Key{
		Coin:     coin,
		Duration: time.Duration(d) * time.Minute,
		Strategy: fmt.Sprintf("%s_%d", string(coin), d),
	}
}

func forCoin(coin model.Coin) func(sgm mlmodel.SegmentConfig) mlmodel.SegmentConfig {
	return func(sgm mlmodel.SegmentConfig) mlmodel.SegmentConfig {
		//sgm[ConfigKey(coin, 15)] = mlmodel.Segments{
		//	Stats:  StatsConfig(0.2),
		//	Model:  ModelConfig(0.61),
		//	Trader: TraderConfig(),
		//}
		sgm[ConfigKey(coin, 30)] = mlmodel.Segments{
			Stats:  StatsConfig(0.2),
			Model:  ModelConfig(0.65),
			Trader: TraderConfig(true),
		}
		return sgm
	}
}

func configML() *mlmodel.Config {

	cfg := mlmodel.SegmentConfig(make(map[model.Key]mlmodel.Segments))

	cfg = cfg.
		AddConfig(forCoin(model.BTC)).
		AddConfig(forCoin(model.DOT)).
		AddConfig(forCoin(model.ETH)).
		AddConfig(forCoin(model.LINK)).
		AddConfig(forCoin(model.SOL)).
		AddConfig(forCoin(model.FLOW)).
		AddConfig(forCoin(model.MATIC)).
		AddConfig(forCoin(model.AAVE)).
		AddConfig(forCoin(model.KSM)).
		AddConfig(forCoin(model.XRP)).
		AddConfig(forCoin(model.ADA)).
		AddConfig(forCoin(model.KAVA))

	return &mlmodel.Config{
		Segments: cfg,
		Position: mlmodel.Position{
			OpenValue:  250,
			StopLoss:   0.02,
			TakeProfit: 0.015,
			TrackingConfig: []*model.TrackingConfig{{
				Duration: 30 * time.Second,
				Samples:  3,
				// TODO : investigate more what this does
				//Threshold: []float64{0.00005, 0.000002},
				//Threshold: []float64{0.00002, 0.000001},
				Threshold: []float64{0.00003, 0.000002},
			}},
		},
		Option: mlmodel.Option{
			Trace: map[string]bool{
				//string(model.BTC): true,
			},
			Log:       true,
			Debug:     true,
			Benchmark: true,
		},
		Buffer: mlmodel.Buffer{
			Interval: 10 * time.Second,
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
