package test

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/algo/processor/trade"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

func TestTrader_Gather(t *testing.T) {

	type test struct {
		transform func(i int) float64
		msgCount  int
		config    trade.Config
	}

	tests := map[string]test{
		"sin": {
			transform: func(i int) float64 {
				return math.Sin(float64(i/10) * 40000)
			},
			msgCount: 13,
			config: trade.Config{
				Coin: "",
				Open: trade.Open{
					Value: 0.1,
				},
				Strategies: []trade.Strategy{
					{
						Sample:      1,
						Probability: 0.9,
					},
				},
			},
		},
		"inc": {
			transform: func(i int) float64 {
				return 40000 + 100*float64(i)
			},
			msgCount: 35,
		},
		"dec": {
			transform: func(i int) float64 {
				return 40000 - 100*float64(i)
			},
			msgCount: 35,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := make(chan *model.Trade)
			st := make(chan *model.Trade)
			client := newMockClient()
			block := api.NewBlock()
			_, statsMessages := run(client, in, st, func(client api.Exchange, user api.User) api.Processor {
				return stats.MultiStats(user)
			})
			out := make(chan *model.Trade)
			cmds, tradeMessages := run(client, st, out, func(client api.Exchange, user api.User) api.Processor {
				return trade.Trade(client, user, block)
			})

			// send the config to the processor
			cmds <- api.Command{
				ID:      1,
				User:    "",
				Content: fmt.Sprintf("?t BTC 10 %f %d", tt.config.Strategies[0].Probability, tt.config.Strategies[0].Sample),
			}

			wg := new(sync.WaitGroup)
			wg.Add(tt.msgCount)
			go logMessages("stats", nil, statsMessages)
			go logMessages("trade", wg, tradeMessages)

			num := 1000
			wg.Add(num)
			go func() {
				start := time.Now()
				for i := 0; i < num; i++ {
					trade := newTrade(model.BTC, tt.transform(i), 1, model.Buy, start.Add(time.Duration(i*15)*time.Second))
					// enable trade to publish messages
					trade.Live = true
					in <- trade
				}
			}()

			go func() {
				for range out {

				}
			}()

			wg.Wait()
		})
	}

}
