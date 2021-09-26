package binance

import (
	"fmt"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestClient_Trades(t *testing.T) {

	client := NewClient(model.BTC, model.ETH).
		Interval(1 * time.Minute)

	//limit := 7999
	//client := NewClient(model.BTC, model.ETH).
	//	Interval(1 * time.Second).
	//	Stop(api.Counter(limit)).
	//	WithRemote(newMockSource("testdata/response-trades"))

	process := make(chan api.Signal)
	source, err := client.Trades(process)
	assert.NoError(t, err)

	counter := make(map[model.Coin]int)

	i := 0
	for trade := range source {
		fmt.Printf("trade = %+v\n", trade)
		coin := trade.Coin
		if _, ok := counter[coin]; !ok {
			counter[coin] = 1
		} else {
			counter[coin]++
		}
		i++
	}

	// total trades consumed , which also caused the stop of the execution
	//assert.Equal(t, limit-1, i)
	// BTC is first so it consumed the full batch
	assert.Equal(t, counter[model.Coin(string(model.BTC))], 4000)
	// the last batch of ETH was interrupted
	assert.Equal(t, counter[model.Coin(string(model.ETH))], 3998)

}
