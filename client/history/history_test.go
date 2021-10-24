package history

import (
	"fmt"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/storage/file/json"

	"github.com/drakos74/free-coin/client/kraken"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestHistoryClient_Trades_Write(t *testing.T) {

	//client := NewClient(model.BTC, model.ETH).
	//	Since(cointime.LastXHours(24)).
	//	Interval(5 * time.Second)

	limit := 7999
	client := New(
		kraken.NewClient(model.BTC, model.ETH).
			Interval(1 * time.Second).
			Stop(api.Counter(limit)).
			WithRemote(kraken.NewMockSource("../kraken/testdata/response-trades")),
	).WithRegistry(json.NewEventRegistry("test"))

	process := make(chan api.Signal)
	source, err := client.Trades(process)
	assert.NoError(t, err)

	counter := make(map[model.Coin]int)

	i := 0
	for trade := range source {
		// receive trade
		fmt.Printf("trade = %+v\n", trade)
		coin := trade.Coin
		if _, ok := counter[coin]; !ok {
			counter[coin] = 1
		} else {
			counter[coin]++
		}
		i++
		// unblock source
		process <- api.Signal{}
	}

	// total trades consumed , which also caused the stop of the execution
	assert.Equal(t, limit-1, i)
	// BTC is first so it consumed the full batch
	assert.Equal(t, 4000, counter[model.Coin(string(model.BTC))])
	// the last batch of ETH was interrupted
	assert.Equal(t, 3998, counter[model.Coin(string(model.ETH))])
}

func TestHistoryClient_Trades_Read(t *testing.T) {
	process := make(chan api.Signal)

	readHistory := New(nil).WithRegistry(json.NewEventRegistry("test")).Reader(&Request{
		Coin: model.BTC,
		To:   time.Now(),
	})

	readSource, err := readHistory.Trades(process)
	assert.NoError(t, err)

	readCounter := make(map[model.Coin]int)

	j := 0
	for trade := range readSource {
		// receive trade
		fmt.Printf("trade = %+v\n", trade)
		coin := trade.Coin
		if _, ok := readCounter[coin]; !ok {
			readCounter[coin] = 1
		} else {
			readCounter[coin]++
		}
		j++
		// unblock source
		process <- api.Signal{}
	}
	// BTC is first so it consumed the full batch
	assert.Equal(t, 4000, readCounter[model.Coin(string(model.BTC))])
}
