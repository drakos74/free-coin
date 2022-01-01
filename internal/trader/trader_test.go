package trader

import (
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/stretchr/testify/assert"
)

func TestTrader_Add(t *testing.T) {

	type test struct {
		openKey       Key
		checkKey      Key
		openOrder     *model.Order
		openPosition  bool
		openPositions int
		close         bool
	}

	tests := map[string]test{
		"no-position": {
			checkKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
		},
		"open-position": {
			openKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
			checkKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
			openOrder:    model.NewOrder(model.BTC).Buy().Market().WithVolume(1),
			openPosition: true,
		},
		"open-position-double": {
			openKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
			checkKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
			openOrder:    model.NewOrder(model.BTC).Buy().Market().WithVolume(1),
			openPosition: true,
		},
		"open-position-with-close": {
			openKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
			checkKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
			openOrder:    model.NewOrder(model.BTC).Buy().Market().WithVolume(1),
			openPosition: true,
			close:        true,
		},
		"other-open-position": {
			openKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
			checkKey: Key{
				Coin:     model.BTC,
				Duration: 10 * time.Minute,
			},
			openOrder:     model.NewOrder(model.BTC).Buy().Market().WithVolume(1),
			openPositions: 1,
		},
		"other-coin-position": {
			openKey: Key{
				Coin:     model.BTC,
				Duration: 5 * time.Minute,
			},
			checkKey: Key{
				Coin:     model.ETH,
				Duration: 10 * time.Minute,
			},
			openOrder: model.NewOrder(model.BTC).Buy().Market().WithVolume(1),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			trader, err := newTrader("id", json.LocalShard(), make(map[model.Coin]map[time.Duration]Settings))
			assert.NoError(t, err)

			if tt.openOrder != nil {
				// lets add an order
				err = trader.add(tt.openKey, model.NewTrackedOrder(model.Key{
					Coin:     tt.openKey.Coin,
					Duration: tt.openKey.Duration,
				}, time.Now(), tt.openOrder.Create()))
				assert.NoError(t, err)
			}

			p, ok, pp := trader.check(tt.checkKey)
			if tt.openPosition {
				assert.True(t, ok)
				assert.Equal(t, tt.openOrder.Type, p.Type)
				assert.Equal(t, tt.openOrder.Volume, p.Volume)
				assert.Equal(t, tt.openOrder.Coin, p.Coin)
				if tt.close {
					err = trader.close(tt.openKey)
					assert.NoError(t, err)
					p, ok, pp = trader.check(tt.openKey)
					assert.False(t, ok)
					assert.Equal(t, 0, len(pp))
				}
			} else {
				assert.False(t, ok)
			}
			assert.Equal(t, tt.openPositions, len(pp))
		})

	}

}
