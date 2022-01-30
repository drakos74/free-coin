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
		config Config
		trades func() []*model.Trade
		pnl    []client.Report
	}

	tests := map[string]test{
		"increasing": {
			config: testUniformML(5, 10, 3, 0.5),
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
		"decreasing": {
			config: testUniformML(5, 10, 3, 0.5),
			trades: testTrades(100, 5, func(i int) float64 {
				return -10.0 * float64(i)
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
					SellAvg: 30400,
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
					Buy:     6,
					BuyAvg:  29500,
					Sell:    5,
					SellAvg: 29600,
					Profit:  5,
				},
				{
					Buy:     6,
					BuyAvg:  29700,
					Sell:    5,
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
					Buy:     8,
					BuyAvg:  29800,
					Sell:    9,
					SellAvg: 30000,
					Profit:  18,
				},
				{
					Buy:     9,
					BuyAvg:  30000,
					Sell:    11,
					SellAvg: 30150,
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

			proc := Processor("", json.LocalShard(), nil, tt.config)

			u, err := local.NewUser("")
			assert.NoError(t, err)

			e := localExchange.NewExchange("")
			exec := proc(u, e)

			chIn := make(chan *model.Trade)
			chOut := make(chan *model.Trade)

			go exec(chIn, chOut)

			start := time.Now().AddDate(1, 0, 0)
			end := time.Now().AddDate(-1, 0, 0)

			pp := make([]float64, 0)
			go func() {
				trades := tt.trades()
				for _, trade := range trades {
					if start.After(trade.Time) {
						start = trade.Time
					}
					if trade.Time.After(end) {
						end = trade.Time
					}
					pp = append(pp, trade.Price)
					chIn <- trade
				}
				close(chIn)
			}()

			// consume the output
			for trade := range chOut {
				fmt.Printf("%v:%v - %+v\n", trade.Time.Hour(), trade.Time.Minute(), trade.Price)
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

func testTrades(s, t int, p func(i int) float64) func() []*model.Trade {
	return func() []*model.Trade {
		trades := make([]*model.Trade, 0)

		now := time.Now()
		for i := 0; i < s; i++ {
			trades = append(trades, &model.Trade{
				Coin:  "BTC",
				Price: 30000.0 + p(i),
				Time:  now.Add(time.Duration(i) * time.Duration(t) * time.Minute),
			})
		}

		return trades
	}
}

func testUniformML(bufferSize, modelSize, features int, precisionThreshold float64) Config {
	cfg := map[model.Key]Segments{
		model.Key{
			Coin:     model.BTC,
			Duration: 15 * time.Minute,
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

	return Config{
		Segments: cfg,
		Position: Position{
			OpenValue:  500,
			StopLoss:   0.01,
			TakeProfit: 0.01,
		},
		Debug:     true,
		Benchmark: false,
	}
}

func testVaryingML(duration, bufferSize, modelSize, features int, precisionThreshold float64) Config {
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

	return Config{
		Segments: cfg,
		Position: Position{
			OpenValue:  500,
			StopLoss:   0.01,
			TakeProfit: 0.01,
		},
		Debug:     true,
		Benchmark: false,
	}
}

func TestSineFunc(t *testing.T) {
	for i := 0; i < 100; i++ {
		x := math.Sin(float64(i) * 0.1)
		fmt.Printf("x = %+v\n", x)
	}
}