package local

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/stretchr/testify/assert"
)

func TestClient_Trades(t *testing.T) {

	// change the default storage directory
	json.DefaultDir = "testdata"
	// delete all contents from the test directory
	err := os.RemoveAll("testdata/table/tmp-shard")
	assert.NoError(t, err)

	type test struct {
		client            *mockSource
		shard             string
		coin              model.Coin
		trades            int
		lastTradeTime     time.Time
		maxUpstreamTrades int
	}

	tests := map[string]test{
		"no-cache": {
			client:            newMockSource(0),
			coin:              model.ETH,
			shard:             "tmp-shard",
			trades:            49 * int(time.Hour.Minutes()),
			lastTradeTime:     time.Unix(0, 0).Add(49 * time.Hour).Add(-1 * time.Second),
			maxUpstreamTrades: 2949,
		},
		"from-cache": {
			client:            newMockSource(0),
			coin:              model.LINK,
			shard:             "shard",
			trades:            47 * int(time.Hour.Minutes()),
			lastTradeTime:     time.Unix(0, 0).Add(30 * time.Hour).Add(-1 * time.Second),
			maxUpstreamTrades: 0,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var since int64 = 0
			client := NewClient(context.Background(), since).
				WithUpstream(func(since int64) (api.Client, error) {
					tt.client.start = cointime.FromNano(since)
					return tt.client, nil
				}).
				WithPersistence(func(shard string) (storage.Persistence, error) {
					return json.NewJsonBlob("table", tt.shard), nil
				})
			trades, err := client.Trades(make(chan struct{}), tt.coin, api.NonStop)
			assert.NoError(t, err)

			// the mock source adds one trade per minute ...
			// the local client hashes by 24 hours ...
			// so we should expect that many trades for the 1 day batch
			// so we put a bit more ... to make sure we can check the stored file later
			var lastTime time.Time
			i := 0
			for trade := range trades {
				lastTime = trade.Time
				i++
				if i > tt.trades {
					// stop processing more if we are over our limit
					break
				}
				// we should get trades for a day ...
			}
			assert.Equal(t, tt.trades+1, i)
			assert.Equal(t, tt.maxUpstreamTrades, tt.client.trades)
			assert.True(t, tt.lastTradeTime.Before(lastTime), fmt.Sprintf("expected %v before %v", tt.lastTradeTime, lastTime))
		})
	}
}

type mockSource struct {
	start  time.Time
	trades int
}

func newMockSource(since int64) *mockSource {
	t := cointime.FromNano(since)
	return &mockSource{start: t}
}

func (m *mockSource) Trades(stop <-chan struct{}, coin model.Coin, stopExecution api.Condition) (model.TradeSource, error) {

	trades := make(chan *model.Trade)

	go func() {
		i := 0
		for {
			select {
			case <-stop:
				close(trades)
				return
			default:
				// generate some trades ... but be careful of the time
				trade := &model.Trade{
					Coin: coin,
					Time: m.start.Add(time.Duration(i) * time.Minute),
				}
				i++
				m.trades++
				trades <- trade
			}
		}
	}()

	return trades, nil
}

func (m *mockSource) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	panic("implement me")
}

func (m *mockSource) OpenPosition(position model.Position) error {
	panic("implement me")
}

func (m *mockSource) ClosePosition(position model.Position) error {
	panic("implement me")
}
