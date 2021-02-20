package processor

import (
	"fmt"
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

	_, in, out, _, msgs := run(MultiStats)
	wg := new(sync.WaitGroup)
	wg.Add(33)
	go func() {
		i := 0
		for msg := range msgs {
			println(fmt.Sprintf("msg = %+v", msg.msg.Text))
			wg.Done()
			i++
		}
	}()

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
