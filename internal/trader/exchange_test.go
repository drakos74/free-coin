package trader

import (
	"testing"
	"time"

	"github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/stretchr/testify/assert"
)

func TestExchangeTrader_CreateOrder(t *testing.T) {

	type signal struct {
		t  model.Type
		c  model.Coin
		d  time.Duration
		p  float64
		ok bool
	}

	type test struct {
		signals []signal
		open    int
		total   int
		wallet  float64
	}

	tests := map[string]test{
		"open-order": {
			signals: []signal{
				{
					model.Buy,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
			},
			open:   1,
			total:  1,
			wallet: -1000,
		},
		"close-order": {
			signals: []signal{
				{
					model.Buy,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
				{
					model.Sell,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
			},
			open:   1,
			total:  3,
			wallet: 1000,
		},
		"close-order-wallet": {
			signals: []signal{
				{
					model.Buy,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
				{
					model.Sell,
					model.BTC,
					time.Minute,
					1100,
					true,
				},
			},
			open:   1,
			total:  3,
			wallet: 1200,
		},
		"close-order-loss": {
			signals: []signal{
				{
					model.Buy,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
				{
					model.Sell,
					model.BTC,
					time.Minute,
					900,
					true,
				},
			},
			open:   1,
			total:  3,
			wallet: 800,
		},
		"extend-order": {
			signals: []signal{
				{
					model.Buy,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
				{
					model.Buy,
					model.BTC,
					time.Minute,
					1000,
					false,
				},
			},
			open:   1,
			total:  1,
			wallet: -1000,
		},
		"order-sequence": {
			signals: []signal{
				{
					model.Buy,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
				{
					model.Sell,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
				{
					model.Buy,
					model.BTC,
					time.Minute,
					1000,
					true,
				},
			},
			open:   1,
			total:  5,
			wallet: -1000,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			exchange := local.NewExchange("")
			trd, err := newTrader("test", json.LocalShard(), make(map[model.Coin]map[time.Duration]Settings))
			assert.NoError(t, err)
			trader := NewExchangeTrader(trd, exchange, nil)

			for _, signal := range tt.signals {
				o, ok, _, err := trader.CreateOrder(model.Key{
					Coin:     signal.c,
					Duration: signal.d,
				}, time.Now(), signal.p, signal.t, true, 1)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, signal.ok, ok)
				if ok {
					assert.Equal(t, o.Type, signal.t)
					assert.Equal(t, o.Coin, signal.c)
				}
			}

			orders := exchange.Orders()
			sum := 0.0
			for _, o := range orders {
				s := o.Volume * o.OpenPrice
				switch o.Type {
				case model.Buy:
					sum -= s
				case model.Sell:
					sum += s
				default:
					t.Fail()
				}
			}
			assert.Equal(t, tt.wallet, sum)

			assert.Equal(t, tt.open, len(trd.positions))
			assert.Equal(t, tt.total, len(orders))
			wallet := exchange.Gather(true)
			assert.Equal(t, tt.total, wallet[model.BTC].Buy+wallet[model.BTC].Sell)
		})
	}

}

// TODO: actually create this test
func TestExchangeTrader_Update(t *testing.T) {

	exchange := local.NewExchange("")
	trd, err := newTrader("test", json.LocalShard(), make(map[model.Coin]map[time.Duration]Settings))
	assert.NoError(t, err)
	trader := NewExchangeTrader(trd, exchange, nil)
	trader.Actions()

}
