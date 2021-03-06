package position

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type test struct {
	config     processor.Close
	positions  []model.Position
	transform  func(i int) float64
	close      bool
	expProfit  float64
	tradeCount int
	update     bool
	cycle      func(tracker *tradePositions, client api.Exchange, i int) *model.Trade
}

func TestTradePosition_DoClose(t *testing.T) {

	tests := map[string]test{
		"continuous-profit": {
			config: processor.Close{
				Profit: processor.Setup{
					Min:   2,
					Trail: 0.15,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func(i int) float64 {
				return 100 + float64(i)
			},
			tradeCount: 100,
		},
		"continuous-profit-no-trail": {
			config: processor.Close{
				Profit: processor.Setup{
					Min: 2,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func(i int) float64 {
				return 100 + float64(i)
			},
			close:      true,
			expProfit:  3,
			tradeCount: 4,
		},
		"profit-close": {
			config: processor.Close{
				Profit: processor.Setup{
					Min:   2,
					Trail: 0.15,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return c + 100
				}
			}(),
			close:      true,
			expProfit:  50,
			tradeCount: 52,
		},
		"profit-decimal-close": {
			config: processor.Close{
				Profit: processor.Setup{
					Min:   2,
					Trail: 0.15,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return c/10 + 100
				}
			}(),
			expProfit:  4.9,
			tradeCount: 53,
			close:      true,
		},
		"profit-decimal-high-trail": {
			config: processor.Close{
				Profit: processor.Setup{
					Min:   2,
					Trail: 1,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return c/10 + 100
				}
			}(),
			expProfit:  4.1,
			tradeCount: 61,
			close:      true,
		},
		"profit-decimal-high-trail-with-update": {
			config: processor.Close{
				Profit: processor.Setup{
					Min:   2,
					Trail: 1,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return c/10 + 100
				}
			}(),
			expProfit:  4.1,
			tradeCount: 61,
			close:      true,
			update:     true,
		},
		"profit-decimal-no-trail": {
			config: processor.Close{
				Profit: processor.Setup{
					Min: 2,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return c/10 + 100
				}
			}(),
			expProfit:  2.1,
			tradeCount: 21,
			close:      true,
		},
		"profit-never-close": { // this will never close, because the margin is too high
			config: processor.Close{
				Profit: processor.Setup{
					Min:   5,
					Trail: 0.15,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 10, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return c/100 + 10
				}
			}(),
			tradeCount: 100,
		},
		"continuous-loss": {
			config: processor.Close{
				Profit: processor.Setup{
					Min: 2,
				},
				Loss: processor.Setup{
					Min:   2,
					Trail: 0.15,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func(i int) float64 {
				return 100 - float64(i)
			},
			expProfit:  -3,
			tradeCount: 4,
			close:      true,
		},
		"continuous-loss-no-trail": { // note : same result
			config: processor.Close{
				Profit: processor.Setup{
					Min: 2,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func(i int) float64 {
				return 100 - float64(i)
			},
			close:      true,
			expProfit:  -3,
			tradeCount: 4,
		},
		"loss-close": { // does not test anything new
			config: processor.Close{
				Profit: processor.Setup{
					Min: 2,
				},
				Loss: processor.Setup{
					Min:   2,
					Trail: 0.15,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return 100 - c
				}
			}(),
			close:      true,
			expProfit:  -3,
			tradeCount: 3,
		},
		"loss-decimal-close": {
			config: processor.Close{
				Profit: processor.Setup{
					Min: 2,
				},
				Loss: processor.Setup{
					Min:   2,
					Trail: 0.15,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return 100 - c/10
				}
			}(),
			expProfit:  -2.1,
			tradeCount: 21,
			close:      true,
		},
		"loss-decimal-high-trail": { // note : has same effect
			config: processor.Close{
				Profit: processor.Setup{
					Min: 2,
				},
				Loss: processor.Setup{
					Min:   2,
					Trail: 10,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return 100 - c/10
				}
			}(),
			expProfit:  -2.1,
			tradeCount: 21,
			close:      true,
		},
		"loss-decimal-no-trail": { // note : has the same effect
			config: processor.Close{
				Profit: processor.Setup{
					Min: 2,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 100, 1),
			},
			transform: func() func(i int) float64 {
				c := 0.0
				return func(i int) float64 {
					if i > 50 {
						c--
					} else {
						c++
					}
					return 100 - c/10
				}
			}(),
			expProfit:  -2.1,
			tradeCount: 21,
			close:      true,
		},
		"loss-never-close": { // this will close immediately, as we have no trail for loss currently
			config: processor.Close{
				Profit: processor.Setup{
					Min:   5,
					Trail: 0.15,
				},
				Loss: processor.Setup{
					Min: 2,
				},
			},
			positions: []model.Position{
				mockPosition(model.Buy, 10, 1),
			},
			transform: func(i int) float64 {
				return 5 + float64(i)
			},
			close:      true,
			expProfit:  -50,
			tradeCount: 1,
		},
	}

	for name, tt := range tests {
		// note ... we run two scenarios
		// - one for none update and many trades
		// - one for update on eahc trade : to check the update does not reset the config
		if tt.update {
			tt.cycle = func(tracker *tradePositions, client api.Exchange, i int) *model.Trade {
				err := tracker.update(client)
				assert.NoError(t, err)
				return mockTrade(tt.transform(i))
			}
		} else {
			tt.cycle = func(tracker *tradePositions, client api.Exchange, i int) *model.Trade {
				return mockTrade(tt.transform(i))
			}
		}
		t.Run(name, func(t *testing.T) {

			config := processor.Config{
				Duration: 10,
				Strategies: []processor.Strategy{
					{
						Close: tt.config,
					},
				},
			}
			tracker := newPositionTracker(storage.NewVoidRegistry(), map[model.Coin]map[time.Duration]processor.Config{
				"": {
					10: config,
				},
			})
			client := mockExchange(tt.positions...)
			err := tracker.update(client)
			assert.NoError(t, err)
			assertPositions(t, tt, tracker, client, tt.cycle)
			if tt.update {
				assert.Equal(t, tt.tradeCount+1, client.(*exchange).requests)
			} else {
				assert.Equal(t, 1, client.(*exchange).requests)
			}
		})
	}
}

func assertPositions(t *testing.T, tt test, tracker *tradePositions, client api.Exchange, makeTrade func(tracker *tradePositions, client api.Exchange, i int) *model.Trade) {
	for i := 0; i < 100; i++ {
		trade := makeTrade(tracker, client, i)
		actions := tracker.checkClose(trade)
		for _, action := range actions {
			if action.doClose {
				assert.True(t, tt.close)
				assert.Equal(t, testCoin, action.key.coin)
				assert.Equal(t, testCoin, action.position.position.Coin)
				assert.Equal(t, testID, action.key.id)
				assert.Equal(t, testID, action.position.position.ID)
				_, profit := action.position.position.Value()
				assert.Equal(t, fmt.Sprintf("%.2f", tt.expProfit), fmt.Sprintf("%.2f", profit))
				assert.Equal(t, tt.tradeCount, i+1)
				return
			}
		}
	}
	// we should have closed a trade by now... or not ?
	assert.False(t, tt.close)
	assert.Equal(t, tt.tradeCount, 100)
}

var (
	testID   = uuid.New().String()
	testCoin = model.Coin("coin")
)

func mockTrade(p float64) *model.Trade {
	return &model.Trade{
		Coin:  testCoin,
		Price: p,
		Live:  true,
	}
}

func mockPosition(t model.Type, p, v float64) model.Position {
	return model.Position{
		Coin:      testCoin,
		ID:        testID,
		OpenPrice: p,
		Volume:    v,
		Type:      t,
	}
}

func mockExchange(positions ...model.Position) api.Exchange {
	return &exchange{positions: model.PositionBatch{
		Positions: positions,
	}}
}

type exchange struct {
	positions model.PositionBatch
	requests  int
}

func (e *exchange) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	e.requests++
	return &e.positions, nil
}

func (e *exchange) OpenOrder(order model.Order) ([]string, error) {
	panic("implement me")
}

func (e *exchange) ClosePosition(position model.Position) error {
	panic("implement me")
}
