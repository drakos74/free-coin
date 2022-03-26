package ml

import (
	"fmt"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/model"
	"github.com/stretchr/testify/assert"
)

const (
	bufferTime     = 3
	halfBufferTime = bufferTime / 2
	value          = 1000
)

func TestStrategy(t *testing.T) {

	now := time.Now()

	type test struct {
		signals map[int]Signal
		trades  map[int]*model.TradeSignal
		action  []bool
	}

	tests := map[string]test{
		"no_trade_time": {
			signals: map[int]Signal{
				0: mockSignal(now, 1000, model.Buy),
			},
			trades: map[int]*model.TradeSignal{
				1: mockTrade(now.Add(halfBufferTime*time.Hour), 1100),
			},
			action: []bool{false, false},
		},
		"no_trade_value": {
			signals: map[int]Signal{
				0: mockSignal(now, 1000, model.Buy),
			},
			trades: map[int]*model.TradeSignal{
				1: mockTrade(now.Add(bufferTime*time.Hour), 900),
			},
			action: []bool{false, false},
		},
		"buy_trade": {
			signals: map[int]Signal{
				0: mockSignal(now, 1000, model.Buy),
			},
			trades: map[int]*model.TradeSignal{
				1: mockTrade(now.Add(bufferTime*time.Hour), 1100),
				2: mockTrade(now.Add((bufferTime+halfBufferTime)*time.Hour), 1100),
			},
			action: []bool{false, true, false},
		},
		"multiple_no-buy_trade": {
			signals: map[int]Signal{
				0: mockSignal(now, 1000, model.Buy),
			},
			trades: map[int]*model.TradeSignal{
				1: mockTrade(now.Add((bufferTime)*time.Hour), 1100),
				2: mockTrade(now.Add((bufferTime+bufferTime)*time.Hour), 1100),
			},
			action: []bool{false, true, false},
		},
		"multiple_buy_trade": {
			signals: map[int]Signal{
				0: mockSignal(now, 1000, model.Buy),
			},
			trades: map[int]*model.TradeSignal{
				1: mockTrade(now.Add(bufferTime*time.Hour), 1100),
				2: mockTrade(now.Add((bufferTime+bufferTime)*time.Hour), 1200),
			},
			action: []bool{false, true, true},
		},
		"confirm_signal": {
			signals: map[int]Signal{
				0: mockSignal(now, 1000, model.Buy),
				1: mockSignal(now.Add(halfBufferTime*time.Hour), 1000, model.Buy),
			},
			trades: map[int]*model.TradeSignal{
				2: mockTrade(now.Add(bufferTime*time.Hour), 1100),
			},
			action: []bool{false, false, true},
		},
		"opposite_signal_stall": {
			signals: map[int]Signal{
				0: mockSignal(now, 1000, model.Buy),
				1: mockSignal(now.Add(1*time.Hour), 1000, model.Sell),
			},
			trades: map[int]*model.TradeSignal{
				2: mockTrade(now.Add(bufferTime*time.Hour), 1100),
				3: mockTrade(now.Add((bufferTime+halfBufferTime)*time.Hour), 1100),
			},
			action: []bool{false, false, false, false},
		},
		"opposite_signal_act": {
			signals: map[int]Signal{
				0: mockSignal(now, 1000, model.Buy),
				1: mockSignal(now.Add(halfBufferTime*time.Hour), 1000, model.Sell),
			},
			trades: map[int]*model.TradeSignal{
				2: mockTrade(now.Add((bufferTime+halfBufferTime)*time.Hour), 900),
			},
			action: []bool{false, false, true},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			strategy := newStrategy(map[model.Key]Segments{
				model.Key{
					Coin: model.BTC,
				}: {
					Trader: Trader{
						BufferTime:     bufferTime,
						PriceThreshold: 50,
					},
				},
			})
			k := model.Key{}
			signalCount := 0
			tradeCount := 0
			for i, b := range tt.action {
				if s, ok := tt.signals[i]; ok {
					signalCount++
					_, act := strategy.signal(k, s)
					assert.Equal(t, b, act, fmt.Sprintf("unexpected action at %d", i))
				}
				if tr, ok := tt.trades[i]; ok {
					tradeCount++
					_, _, _, act := strategy.trade(k, tr.Tick)
					assert.Equal(t, b, act, fmt.Sprintf("unexpected action at %d", i))
				}
			}
			// check we asserted all available actions
			assert.Equal(t, len(tt.trades), tradeCount)
			assert.Equal(t, len(tt.signals), signalCount)
		})
	}
}

func mockSignal(t time.Time, p float64, tt model.Type) Signal {
	return Signal{
		Key: model.Key{
			Coin:     model.BTC,
			Duration: 0,
		},
		Time:  t,
		Price: p,
		Type:  tt,
	}
}

func mockTrade(t time.Time, p float64) *model.TradeSignal {
	return &model.TradeSignal{
		Coin: model.BTC,
		Tick: model.Tick{
			Level: model.Level{
				Price: p,
			},
			Type:   0,
			Time:   t,
			Active: false,
		},
	}
}
