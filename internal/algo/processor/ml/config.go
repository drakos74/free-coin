package ml

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor/ml/net"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/model"
)

func CoinConfig(coins map[model.Coin]mlmodel.ConfigSegment) *mlmodel.Config {
	cfg := mlmodel.SegmentConfig(make(map[model.Key]mlmodel.Segments))

	for coin, gen := range coins {
		cfg = cfg.
			AddConfig(gen(coin))
	}

	return &mlmodel.Config{
		Segments: cfg,
		Position: mlmodel.Position{
			OpenValue:  100,
			StopLoss:   0.020,
			TakeProfit: 0.020,
			TrackingConfig: []*model.TrackingConfig{{
				Duration: 30 * time.Second,
				Samples:  5,
				// TODO : investigate more what this does
				//Threshold: []float64{0.00005, 0.000002},
				//Threshold: []float64{0.00002, 0.000001},
				Threshold: []float64{0.00001, 0.000001},
			}},
		},
		Option: mlmodel.Option{
			Trace: map[string]bool{
				//string(model.AllCoins): true,
				//string(model.BTC): true,
			},
			Log:       false,
			Debug:     true,
			Benchmark: true,
		},
		Buffer: mlmodel.Buffer{
			Interval: 10 * time.Second,
			History:  true,
		},
		Segment: mlmodel.Buffer{
			Interval: 15 * time.Minute,
			History:  true,
		},
	}
}

func WithConfig(coin map[model.Coin]bool) *mlmodel.Config {
	cfg := make(map[model.Coin]mlmodel.ConfigSegment)
	for c, live := range coin {
		cfg[c] = func(coin model.Coin) func(cfg mlmodel.SegmentConfig) mlmodel.SegmentConfig {
			return ForCoin(coin, live)
		}
	}
	return CoinConfig(cfg)
}

func Config(coin ...model.Coin) *mlmodel.Config {
	cfg := make(map[model.Coin]mlmodel.ConfigSegment)
	if len(coin) > 0 {
		for _, c := range coin {
			cfg[c] = func(coin model.Coin) func(cfg mlmodel.SegmentConfig) mlmodel.SegmentConfig {
				return ForCoin(coin, true)
			}
		}
	} else {
		return Config(
			model.BTC,
			model.DOT,
			model.ETH,
			model.LINK,
			model.SOL,
			//model.FLOW,
			model.MATIC,
			model.AAVE,
			//model.KSM,
			model.XRP,
			//model.ADA,
			model.KAVA,
		)
	}
	return CoinConfig(cfg)
}

func ModelConfig(precision float64) mlmodel.Model {
	return mlmodel.Model{
		BufferSize: 64,
		Threshold:  precision,
		Size:       make([]int, 128),
		Features:   make([]int, 8),
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

func ForCoin(coin model.Coin, live bool) func(sgm mlmodel.SegmentConfig) mlmodel.SegmentConfig {
	return func(sgm mlmodel.SegmentConfig) mlmodel.SegmentConfig {
		sgm[ConfigKey(coin, 15)] = defaultConfig(live)
		return sgm
	}
}

func defaultConfig(live bool) mlmodel.Segments {
	return mlmodel.Segments{
		Stats: mlmodel.Stats{
			LookBack:  8,
			LookAhead: 3,
			Gap:       0.5,
			Live:      live,
			Model: []mlmodel.Model{
				{
					Detail: mlmodel.Detail{
						Type: net.GRU_KEY,
						Hash: "gru_1_64_1",
					},
					Size:         []int{1, 64, 1},
					LearningRate: 0.01,
					Threshold:    0,
					MaxEpochs:    100,
					Spread:       1,
				},
				{
					Detail: mlmodel.Detail{
						Type: net.POLY_KEY,
						Hash: "x2",
					},
					BufferSize: 2,
					Features:   []int{2, 1},
					Spread:     1,
					Multi:      true,
				},
				{
					Detail: mlmodel.Detail{
						Type: net.POLY_KEY,
						Hash: "x3",
					},
					BufferSize: 4,
					Features:   []int{3, 1},
					Spread:     1,
					Multi:      true,
				},
				{
					Detail: mlmodel.Detail{
						Type: net.POLY_KEY,
						Hash: "x1",
					},
					BufferSize: 2,
					Features:   []int{1, 1},
					Spread:     1,
					Multi:      true,
				},
				{
					Detail: mlmodel.Detail{
						Type: net.HMM_KEY,
						Hash: "hmm_5_2",
					},
					Features:   []int{5, 2},
					BufferSize: 0,
					Spread:     1,
				},
				// TODO : fix forest does not work currently
				//{
				//	Detail: mlmodel.Detail{
				//		Type: net.FOREST_KEY,
				//	},
				//	BufferSize: 100,
				//	Size:       []int{100},
				//	Features:   []int{8},
				//	Spread:     0.5,
				//},
			},
		},
		Trader: TraderConfig(false),
	}
}
