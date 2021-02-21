package processor

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/model"
)

func TestStats_TradeProcessing(t *testing.T) {
	testTradeProcessing(t, MultiStats)
}

func TestStats_Gather(t *testing.T) {

	in := make(chan *model.Trade)
	out := make(chan *model.Trade)

	_, _, msgs := run(in, out, MultiStats)
	wg := new(sync.WaitGroup)
	wg.Add(33)
	go logMessages("stats", wg, msgs)

	num := 1000
	wg.Add(num)
	go func() {
		start := time.Now()
		for i := 0; i < num; i++ {
			trade := newTrade(model.BTC, math.Sin(float64(i/10)*40000), 1, model.Buy, start.Add(time.Duration(i*15)*time.Second))
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

}
