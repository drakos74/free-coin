package processor

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/api"

	"github.com/drakos74/free-coin/internal/model"
)

func TestTrader_Gather(t *testing.T) {

	type test struct {
		transform func(i int) float64
	}

	tests := map[string]test{
		"sin": {
			transform: func(i int) float64 {
				return math.Sin(float64(i/10) * 40000)
			},
		},
		"inc": {
			transform: func(i int) float64 {
				return 40000 + 100*float64(i)
			},
		},
		"dec": {
			transform: func(i int) float64 {
				return 40000 - 100*float64(i)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := make(chan *model.Trade)
			st := make(chan *model.Trade)
			signals := make(chan api.Signal)
			_, _, statsMessages := run(in, st, func(client api.Exchange, user api.User) api.Processor {
				return MultiStats(client, user, signals)
			})
			out := make(chan *model.Trade)
			_, _, tradeMessages := run(st, out, func(client api.Exchange, user api.User) api.Processor {
				return Trade(client, user, signals)
			})
			wg := new(sync.WaitGroup)
			wg.Add(100)
			go logMessages("stats", wg, statsMessages)
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
					wg.Done()
				}
			}()

			wg.Wait()
		})
	}

}
