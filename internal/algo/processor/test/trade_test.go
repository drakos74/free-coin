package test

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor/position"

	"github.com/rs/zerolog/log"

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
			msgCount: 22,
		},
		"dec": {
			transform: func(i int) float64 {
				return 40000 - 10*float64(i)
			},
			msgCount: 18,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := make(chan *model.Trade)
			st := make(chan *model.Trade)

			block := api.NewBlock()
			client := newMockClient()

			// run the stats processor
			_, statsMessages := run(client, in, st, func(client api.Exchange, user api.User) api.Processor {
				return stats.MultiStats(user, stats.Config{
					Coin:     "LINK",
					Duration: 10,
					Order:    "O10",
					Targets: []stats.Target{
						{
							LookBack:  2,
							LookAhead: 1,
						},
						{
							LookBack:  3,
							LookAhead: 1,
						},
					},
				},
					stats.Config{
						Coin:     "XRP",
						Duration: 10,
						Order:    "O2",
						Targets: []stats.Target{
							{
								LookBack:  2,
								LookAhead: 1,
							},
							{
								LookBack:  3,
								LookAhead: 1,
							},
						},
					})
			})
			// run the position processor to unblock the trade processor when making orders
			run(client, in, st, func(client api.Exchange, user api.User) api.Processor {
				return position.Position(client, user, block, true, position.Config{
					Profit: position.Setup{},
					Loss:   position.Setup{},
				})
			})
			// run the trade processor
			out := make(chan *model.Trade)
			_, tradeMessages := run(client, st, out, func(client api.Exchange, user api.User) api.Processor {
				return trade.Trade(client, user, block, trade.Config{
					Open: trade.Open{
						Value: 1, // TODO : check with this missing
					},
					Strategies: []trade.Strategy{
						{
							Name:        trade.NumericStrategy,
							Target:      10,
							Probability: 0.5,
							Sample:      1,
						},
					},
				})
			})

			// send the config to the processor
			//cmds <- api.Command{
			//	ID:      1,
			//	User:    "",
			//	Content: fmt.Sprintf("?t BTC 10 %f %d", tt.config.Strategies[0].Probability, tt.config.Strategies[0].Sample),
			//}

			wg := new(sync.WaitGroup)
			wg.Add(tt.msgCount)
			go consumeMessages("stats", nil, statsMessages)
			go consumeMessages("trade", wg, tradeMessages)

			num := 1000
			// 1000 / 10 min -> 25 stats events expected
			wg.Add(num)
			go func() {
				start := time.Now()
				for i := 0; i < num; i++ {
					trade := newTrade(model.LINK, tt.transform(i), 1, model.Buy, start.Add(time.Duration(i*15)*time.Second))
					// enable trade to publish messages
					trade.Live = true
					in <- trade
				}
			}()

			go func() {
				i := 0
				for outTrade := range out {
					log.Trace().Int("i", i).Str("trade", fmt.Sprintf("%+v", outTrade)).Msg("out")
					wg.Done()
					i++
				}
			}()

			wg.Wait()
			println(fmt.Sprintf("done = %+v", num))
		})
	}

}
