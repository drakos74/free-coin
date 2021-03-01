package processor

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor/stats"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

func TestStats_TradeProcessing(t *testing.T) {
	testTradeProcessing(t, testMultiStats())
}

func TestStats_Gather(t *testing.T) {

	type test struct {
		transform func(i int) float64
		msgCount  int
	}

	tests := map[string]test{
		"sin": {
			transform: func(i int) float64 {
				return math.Sin(float64(i/10) * 40000)
			},
			msgCount: 35,
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
			out := make(chan *model.Trade)
			client := newMockClient()

			_, msgs := run(client, in, out, testMultiStats())
			wg := new(sync.WaitGroup)
			wg.Add(tt.msgCount)
			go logMessages("stats", wg, msgs)

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

func testMultiStats() func(client api.Exchange, user api.User) api.Processor {
	return func(client api.Exchange, user api.User) api.Processor {
		signal := make(chan model.Signal)
		go func() {
			for range signal {
				// nothing to do just consume, so that the stats processor can proceed
			}
		}()
		return stats.MultiStats(user)
	}
}
