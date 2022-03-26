package processor

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// TradesBuffer is a buffer for trades for the given time range.
type TradesBuffer struct {
	size     int
	duration time.Duration
	windows  map[string]buffer.HistoryWindow
}

// NewBuffer creates a new trade buffer.
func NewBuffer(duration time.Duration) *TradesBuffer {
	return &TradesBuffer{
		windows:  make(map[string]buffer.HistoryWindow),
		duration: duration,
		size:     1,
	}
}

// Push adds an element ot the buffer, if the time range limitations have been reached it will return
// the aggregated trade and the true flag.
func (tb *TradesBuffer) Push(trade *model.TradeSignal) (*model.TradeSignal, bool) {
	if _, ok := tb.windows[string(trade.Coin)]; !ok {
		tb.windows[string(trade.Coin)] = buffer.NewHistoryWindow(tb.duration, tb.size)
	}
	if bucket, ok := tb.windows[string(trade.Coin)].Push(trade.Meta.Time, trade.Tick.Price, trade.Tick.Volume); ok {
		t := model.SignedType(bucket.Values().Stats()[0].Ratio())
		trade = &model.TradeSignal{
			Coin: trade.Coin,
			Tick: model.Tick{
				Level: model.Level{
					Price:  bucket.Values().Stats()[0].Avg(),
					Volume: bucket.Values().Stats()[1].Avg(),
				},
				Type:   t,
				Time:   bucket.Time,
				Active: true,
			},
			Meta: model.Meta{
				Live: true,
				Time: trade.Meta.Time,
			},
		}
		return trade, true
	}
	return nil, false
}

// SignalBuffer is a consistent signal source that will emit a signal each pre-specified interval.
type SignalBuffer struct {
	duration time.Duration
	windows  map[string]*buffer.IntervalWindow
	trades   chan *model.TradeSignal
}

// NewSignalBuffer creates a new trade buffer.
func NewSignalBuffer(duration time.Duration) (*SignalBuffer, <-chan *model.TradeSignal) {
	trades := make(chan *model.TradeSignal)
	sb := &SignalBuffer{
		windows:  make(map[string]*buffer.IntervalWindow),
		duration: duration,
		trades:   trades,
	}
	return sb, trades
}

// Push adds an element ot the buffer.
func (sb *SignalBuffer) Push(trade *model.TradeSignal) {
	coin := string(trade.Coin)
	if _, ok := sb.windows[coin]; !ok {
		bf, trades := buffer.NewIntervalWindow(coin, 2, sb.duration)
		sb.windows[coin] = bf
		// start consuming for the new created window
		go func(coin model.Coin, signals chan<- *model.TradeSignal) {
			for bucket := range trades {
				fmt.Printf("bucket = %+v - %+v\n", coin, bucket.Time)
				signal := &model.TradeSignal{
					Coin: coin,
					Tick: model.Tick{
						Time:   bucket.Time.Add(bucket.Duration),
						Active: bucket.OK,
					},
					Meta: model.Meta{
						Time: bucket.Time,
						Live: bucket.OK,
					},
				}
				if bucket.OK {
					signal.Tick.Level = model.Level{
						Price:  bucket.Stats[0].Avg(),
						Volume: bucket.Stats[1].Avg(),
					}
				}
				signals <- signal
			}
		}(trade.Coin, sb.trades)
	}
	sb.windows[coin].Push(trade.Meta.Time, trade.Tick.Price, trade.Tick.Volume)
}

func (sb *SignalBuffer) Close() {
	for coin, ch := range sb.windows {
		err := ch.Close()
		if err != nil {
			log.Err(err).Str("coin", coin).Msg("error closing buffer")
		}
	}
}
