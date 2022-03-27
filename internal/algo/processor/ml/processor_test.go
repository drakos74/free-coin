package ml

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/drakos74/free-coin/client"
	localExchange "github.com/drakos74/free-coin/client/local"
	coin_math "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/user/local"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func TestProcessor(t *testing.T) {

	type test struct {
		config *Config
		trades func() []*model.TradeSignal
		pnl    []client.Report
	}

	tests := map[string]test{
		"increasing": {
			config: testUniformML(5, 10, 3, 0.5),
			trades: testTrades(500, 5, func(i int) float64 {
				return 10.0 * float64(i)
			}),
			pnl: []client.Report{
				{
					Buy:     1,
					BuyAvg:  30300,
					Sell:    0,
					SellAvg: 0,
					Profit:  10,
				},
				{
					Buy:     1,
					BuyAvg:  30500,
					Sell:    0,
					SellAvg: 0,
					Profit:  15,
				},
			},
		},
		"decreasing": {
			config: testUniformML(5, 10, 3, 0.5),
			trades: testTrades(100, 5, func(i int) float64 {
				return 30000 - 10.0*float64(i)
			}),
			pnl: []client.Report{
				{
					Buy:     0,
					BuyAvg:  0,
					Sell:    1,
					SellAvg: 29600,
					Profit:  10,
				},
				{
					Buy:     0,
					BuyAvg:  0,
					Sell:    1,
					SellAvg: 29700,
					Profit:  15,
				},
			},
		},
		"up-and-down": {
			config: testVaryingML(15, 3, 10, 3, 0.5),
			trades: testTrades(100, 5, func(i int) float64 {
				if i <= 50 {
					return 10.0 * float64(i)
				}
				return 10.0*50.0 - 10.0*float64(i-50)
			}),
			pnl: []client.Report{
				{
					Buy:     1,
					BuyAvg:  30200,
					Sell:    2,
					SellAvg: 30300,
					Profit:  5,
				},
				{
					Buy:     1,
					BuyAvg:  30400,
					Sell:    2,
					SellAvg: 30500,
					Profit:  10,
				},
			},
		},
		"down-and-up": {
			config: testVaryingML(15, 3, 10, 3, 0.5),
			trades: testTrades(100, 5, func(i int) float64 {
				if i <= 50 {
					return -10.0 * float64(i)
				}
				return -10.0*50.0 + 10.0*float64(i-50)
			}),
			pnl: []client.Report{
				{
					Buy:     2,
					BuyAvg:  29500,
					Sell:    1,
					SellAvg: 29600,
					Profit:  5,
				},
				{
					Buy:     3,
					BuyAvg:  29700,
					Sell:    2,
					SellAvg: 29800,
					Profit:  10,
				},
			},
		},
		"sine-high-range": {
			config: testVaryingML(15, 3, 10, 3, 0.5),
			trades: testTrades(100, 5, func(i int) float64 {
				return 500 * coin_math.SineEvolve(i, 0.1)
			}),
			pnl: []client.Report{
				{
					Buy:     2,
					BuyAvg:  29600,
					Sell:    3,
					SellAvg: 30000,
					Profit:  18,
				},
				{
					Buy:     3,
					BuyAvg:  30000,
					Sell:    4,
					SellAvg: 30250,
					Profit:  25,
				},
			},
		},
		"sine-low-range": { // TODO : this produces loss
			config: testVaryingML(15, 3, 10, 3, 0.5),
			trades: testTrades(100, 5, func(i int) float64 {
				return 100 * coin_math.SineEvolve(i, 0.1)
			}),
			pnl: []client.Report{
				{
					Buy:     4,
					BuyAvg:  29900,
					Sell:    6,
					SellAvg: 29900,
					Profit:  0,
				},
				{
					Buy:     6,
					BuyAvg:  30100,
					Sell:    7,
					SellAvg: 30100,
					Profit:  3,
				},
			},
		},
		"sine-high-vol": {
			config: testVaryingML(15, 3, 5, 3, 0.5),
			trades: testTrades(100, 5, func(i int) float64 {
				return 100 * coin_math.SineEvolve(i, 0.1)
			}),
			pnl: []client.Report{
				{
					Buy:     4,
					BuyAvg:  29900,
					Sell:    6,
					SellAvg: 29900,
					Profit:  0,
				},
				{
					Buy:     6,
					BuyAvg:  30100,
					Sell:    7,
					SellAvg: 30100,
					Profit:  3,
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			proc := Processor("", json.LocalShard(), json.EventRegistry("ml-trade-registry"), nil, tt.config)

			u, err := local.NewUser("")
			assert.NoError(t, err)

			e := localExchange.NewExchange("")
			exec := proc(u, e)

			chIn := make(chan *model.TradeSignal)
			chOut := make(chan *model.TradeSignal)

			go exec(chIn, chOut)

			start := time.Now().AddDate(1, 0, 0)
			end := time.Now().AddDate(-1, 0, 0)

			pp := make([]float64, 0)
			go func() {
				trades := tt.trades()
				for _, trade := range trades {
					if start.After(trade.Tick.Time) {
						start = trade.Tick.Time
					}
					if trade.Tick.Time.After(end) {
						end = trade.Tick.Time
					}
					pp = append(pp, trade.Tick.Price)
					chIn <- trade
					time.Sleep(500 * time.Millisecond)
				}
				close(chIn)
			}()

			// consume the output
			for trade := range chOut {
				fmt.Printf("%v:%v - %+v\n", trade.Tick.Time.Hour(), trade.Tick.Time.Minute(), trade.Tick.Price)
				e.Process(trade)
			}

			report := e.Gather(true)

			pnl := report[model.BTC]

			assert.True(t, pnl.Buy >= tt.pnl[0].Buy, fmt.Sprintf("Buy min %d >= %d", pnl.Buy, tt.pnl[0].Buy))
			assert.True(t, pnl.Buy <= tt.pnl[1].Buy, fmt.Sprintf("Buy max %d <= %d", pnl.Buy, tt.pnl[1].Buy))

			assert.True(t, pnl.BuyAvg >= tt.pnl[0].BuyAvg, fmt.Sprintf("BuyAvg min %f >= %f", pnl.BuyAvg, tt.pnl[0].BuyAvg))
			assert.True(t, pnl.BuyAvg <= tt.pnl[1].BuyAvg, fmt.Sprintf("BuyAvg max %f <= %f", pnl.BuyAvg, tt.pnl[1].BuyAvg))

			assert.True(t, pnl.Sell >= tt.pnl[0].Sell, fmt.Sprintf("Sell min %d >= %d", pnl.Sell, tt.pnl[0].Sell))
			assert.True(t, pnl.Sell <= tt.pnl[1].Sell, fmt.Sprintf("Sell max %d <= %d", pnl.Sell, tt.pnl[1].Sell))

			assert.True(t, pnl.SellAvg >= tt.pnl[0].SellAvg, fmt.Sprintf("SellAvg min %f >= %f", pnl.SellAvg, tt.pnl[0].SellAvg))
			assert.True(t, pnl.SellAvg <= tt.pnl[1].SellAvg, fmt.Sprintf("SellAvg max %f <= %f", pnl.SellAvg, tt.pnl[1].SellAvg))

			assert.True(t, pnl.Profit >= tt.pnl[0].Profit, fmt.Sprintf("Profit min %f >= %f", pnl.Profit, tt.pnl[0].Profit))
			assert.True(t, pnl.Profit <= tt.pnl[1].Profit, fmt.Sprintf("Profit max %f <= %f", pnl.Profit, tt.pnl[1].Profit))

			fmt.Printf("pnl = %+v\n", pnl)

			fmt.Printf("start = %+v\n", start)
			fmt.Printf("end = %+v\n", end)

			coin_math.FFT(pp)

		})
	}
}

func TestMultiStrategyProcessor(t *testing.T) {

	type test struct {
		config *Config
		trades func() []*model.TradeSignal
		pnl    []client.Report
	}

	tests := map[string]test{
		"increasing": {
			config: testMultiML(5, 10, 3, 0.5),
			trades: testTrades(100, 5, func(i int) float64 {
				return 10.0 * float64(i)
			}),
			pnl: []client.Report{
				{
					Buy:     1,
					BuyAvg:  30300,
					Sell:    0,
					SellAvg: 0,
					Profit:  10,
				},
				{
					Buy:     1,
					BuyAvg:  30500,
					Sell:    0,
					SellAvg: 0,
					Profit:  15,
				},
			},
		},
		"sine-high-vol-15": {
			config: testMultiVaryingML(3, 5, 3, 0.5, 15, 20, 30),
			trades: testTrades(100, 5, func(i int) float64 {
				return 100 * coin_math.SineEvolve(i, 0.1)
			}),
			pnl: []client.Report{
				{
					Buy:     2,
					BuyAvg:  29900,
					Sell:    1,
					SellAvg: 29900,
					Profit:  -4,
				},
				{
					Buy:     2,
					BuyAvg:  30100,
					Sell:    1,
					SellAvg: 30100,
					Profit:  0,
				},
			},
		},
		"sine-high-vol-30": {
			config: testMultiVaryingML(3, 5, 3, 0.5, 30, 20, 15),
			trades: testTrades(100, 5, func(i int) float64 {
				return 100 * coin_math.SineEvolve(i, 0.1)
			}),
			pnl: []client.Report{
				{
					Buy:     2,
					BuyAvg:  29900,
					Sell:    1,
					SellAvg: 29900,
					Profit:  -6,
				},
				{
					Buy:     2,
					BuyAvg:  30100,
					Sell:    1,
					SellAvg: 30100,
					Profit:  0,
				},
			},
		},
		"sine-high-vol-15+": {
			config: testMultiVaryingML(3, 5, 3, 0.5, 15, 20, 25, 30, 35),
			trades: testTrades(1000, 5, coin_math.VaryingSine(30000, 5000, 0.3)),
			pnl: []client.Report{
				{
					Buy:     2,
					BuyAvg:  29900,
					Sell:    1,
					SellAvg: 29900,
					Profit:  -6,
				},
				{
					Buy:     2,
					BuyAvg:  30100,
					Sell:    1,
					SellAvg: 30100,
					Profit:  0,
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			proc := Processor("", json.LocalShard(), json.EventRegistry("ml-trade-registry"), nil, tt.config)

			u, err := local.NewUser("")
			assert.NoError(t, err)

			e := localExchange.NewExchange("")
			exec := proc(u, e)

			chIn := make(chan *model.TradeSignal)
			chOut := make(chan *model.TradeSignal)

			go exec(chIn, chOut)

			start := time.Now().AddDate(1, 0, 0)
			end := time.Now().AddDate(-1, 0, 0)

			pp := make([]float64, 0)
			go func() {
				trades := tt.trades()
				for _, trade := range trades {
					if start.After(trade.Tick.Time) {
						start = trade.Tick.Time
					}
					if trade.Tick.Time.After(end) {
						end = trade.Tick.Time
					}
					pp = append(pp, trade.Tick.Price)
					chIn <- trade
				}
				close(chIn)
			}()

			// consume the output
			for trade := range chOut {
				fmt.Printf("%v:%v - %+v\n", trade.Tick.Time.Hour(), trade.Tick.Time.Minute(), trade.Tick.Price)
				e.Process(trade)
			}

			report := e.Gather(true)

			pnl := report[model.BTC]

			assert.True(t, pnl.Buy >= tt.pnl[0].Buy, fmt.Sprintf("Buy min %d >= %d", pnl.Buy, tt.pnl[0].Buy))
			assert.True(t, pnl.Buy <= tt.pnl[1].Buy, fmt.Sprintf("Buy max %d <= %d", pnl.Buy, tt.pnl[1].Buy))

			assert.True(t, pnl.BuyAvg >= tt.pnl[0].BuyAvg, fmt.Sprintf("BuyAvg min %f >= %f", pnl.BuyAvg, tt.pnl[0].BuyAvg))
			assert.True(t, pnl.BuyAvg <= tt.pnl[1].BuyAvg, fmt.Sprintf("BuyAvg max %f <= %f", pnl.BuyAvg, tt.pnl[1].BuyAvg))

			assert.True(t, pnl.Sell >= tt.pnl[0].Sell, fmt.Sprintf("Sell min %d >= %d", pnl.Sell, tt.pnl[0].Sell))
			assert.True(t, pnl.Sell <= tt.pnl[1].Sell, fmt.Sprintf("Sell max %d <= %d", pnl.Sell, tt.pnl[1].Sell))

			assert.True(t, pnl.SellAvg >= tt.pnl[0].SellAvg, fmt.Sprintf("SellAvg min %f >= %f", pnl.SellAvg, tt.pnl[0].SellAvg))
			assert.True(t, pnl.SellAvg <= tt.pnl[1].SellAvg, fmt.Sprintf("SellAvg max %f <= %f", pnl.SellAvg, tt.pnl[1].SellAvg))

			assert.True(t, pnl.Profit >= tt.pnl[0].Profit, fmt.Sprintf("Profit min %f >= %f", pnl.Profit, tt.pnl[0].Profit))
			assert.True(t, pnl.Profit <= tt.pnl[1].Profit, fmt.Sprintf("Profit max %f <= %f", pnl.Profit, tt.pnl[1].Profit))

			fmt.Printf("pnl = %+v\n", pnl)

			fmt.Printf("start = %+v\n", start)
			fmt.Printf("end = %+v\n", end)

			coin_math.FFT(pp)

		})
	}
}

func testTrades(s, t int, g coin_math.Generator) func() []*model.TradeSignal {
	return func() []*model.TradeSignal {
		trades := make([]*model.TradeSignal, 0)

		now := time.Now()
		for i := 0; i < s; i++ {
			tt := now.Add(time.Duration(i) * time.Duration(t) * time.Minute)
			trades = append(trades, &model.TradeSignal{
				Coin: "BTC",
				Tick: model.Tick{
					Level: model.Level{
						Price:  g(i),
						Volume: 1,
					},
					Time:   tt,
					Active: true,
				},
			})
		}

		return trades
	}
}

func testUniformML(bufferSize, modelSize, features int, precisionThreshold float64) *Config {
	cfg := map[model.Key]Segments{
		model.Key{
			Coin:     model.BTC,
			Duration: 5 * time.Second,
			Strategy: "default",
		}: {
			Stats: Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.05,
			},
			Model: Model{
				BufferSize:         bufferSize,
				PrecisionThreshold: precisionThreshold,
				ModelSize:          modelSize,
				Features:           features,
			},
		},
	}

	return &Config{
		Segments: cfg,
		Position: Position{
			OpenValue:  500,
			StopLoss:   0.01,
			TakeProfit: 0.01,
		},
		Option: Option{
			Debug:     true,
			Benchmark: false,
			Test:      true,
		},
		Buffer: Buffer{
			Interval: time.Second,
		},
	}
}

func testMultiML(bufferSize, modelSize, features int, precisionThreshold float64) *Config {
	cfg := map[model.Key]Segments{
		model.Key{
			Coin:     model.BTC,
			Duration: 15 * time.Minute,
			Strategy: "15",
		}: {
			Stats: Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.05,
			},
			Model: Model{
				BufferSize:         bufferSize,
				PrecisionThreshold: precisionThreshold,
				ModelSize:          modelSize,
				Features:           features,
			},
			Trader: Trader{Weight: 1},
		},
		model.Key{
			Coin:     model.BTC,
			Duration: 20 * time.Minute,
			Strategy: "20",
		}: {
			Stats: Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.05,
			},
			Model: Model{
				BufferSize:         bufferSize,
				PrecisionThreshold: precisionThreshold,
				ModelSize:          modelSize,
				Features:           features,
			},
			Trader: Trader{Weight: 0},
		},
		model.Key{
			Coin:     model.BTC,
			Duration: 30 * time.Minute,
			Strategy: "30",
		}: {
			Stats: Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.05,
			},
			Model: Model{
				BufferSize:         bufferSize,
				PrecisionThreshold: precisionThreshold,
				ModelSize:          modelSize,
				Features:           features,
			},
			Trader: Trader{Weight: 0},
		},
	}

	return &Config{
		Segments: cfg,
		Position: Position{
			OpenValue:  500,
			StopLoss:   0.01,
			TakeProfit: 0.01,
		},
		Option: Option{
			Debug:     true,
			Benchmark: false,
			Test:      true,
		},
	}
}

func testVaryingML(duration, bufferSize, modelSize, features int, precisionThreshold float64) *Config {
	cfg := map[model.Key]Segments{
		model.Key{
			Coin:     model.BTC,
			Duration: time.Duration(duration) * time.Minute,
			Strategy: "default",
		}: {
			Stats: Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.05,
			},
			Model: Model{
				BufferSize:         bufferSize,
				PrecisionThreshold: precisionThreshold,
				ModelSize:          modelSize,
				Features:           features,
			},
		},
	}

	return &Config{
		Segments: cfg,
		Position: Position{
			OpenValue:  500,
			StopLoss:   0.01,
			TakeProfit: 0.01,
		},
		Option: Option{
			Debug:     true,
			Benchmark: false,
			Test:      true,
		},
	}
}

func testMultiVaryingML(bufferSize, modelSize, features int, precisionThreshold float64, duration ...int) *Config {
	cfg := make(map[model.Key]Segments)
	w := true
	for _, d := range duration {
		key := model.Key{
			Coin:     model.BTC,
			Duration: time.Duration(d) * time.Minute,
			Strategy: fmt.Sprintf("%d", d),
		}
		segment := Segments{
			Stats: Stats{
				LookBack:  5,
				LookAhead: 1,
				Gap:       0.05,
			},
			Model: Model{
				BufferSize:         bufferSize,
				PrecisionThreshold: precisionThreshold,
				ModelSize:          modelSize,
				Features:           features,
			},
		}
		if w {
			segment.Trader = Trader{
				Weight: 1,
			}
			w = false
		}
		cfg[key] = segment
	}

	return &Config{
		Segments: cfg,
		Position: Position{
			OpenValue:  500,
			StopLoss:   0.01,
			TakeProfit: 0.01,
		},
		Option: Option{
			Debug:     true,
			Benchmark: false,
			Test:      true,
		},
	}
}

func TestSineFunc(t *testing.T) {
	for i := 0; i < 100; i++ {
		x := math.Sin(float64(i) * 0.1)
		fmt.Printf("x = %+v\n", x)
	}
}
