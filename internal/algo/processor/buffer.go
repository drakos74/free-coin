package processor

import (
	"time"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
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
	}

	return sb, trades
}

// Push adds an element ot the buffer.
func (sb *SignalBuffer) Push(trade *model.TradeSignal) {
	if _, ok := sb.windows[string(trade.Coin)]; !ok {
		bf, trades := buffer.NewIntervalWindow(2, sb.duration)
		sb.windows[string(trade.Coin)] = bf

		go func() {
			for bucket := range trades {
				sb.trades <- &model.TradeSignal{
					Coin: trade.Coin,
					Tick: model.Tick{
						Level: model.Level{
							Price:  bucket.Stats[0].Avg(),
							Volume: bucket.Stats[1].Avg(),
						},
						Time:   bucket.Time.Add(bucket.Duration),
						Active: bucket.OK,
					},
					Meta: model.Meta{
						Live: bucket.OK,
						Time: trade.Meta.Time,
					},
				}
			}
		}()

	}
	sb.windows[string(trade.Coin)].Push(trade.Meta.Time, trade.Tick.Price, trade.Tick.Volume)
}

func (sb *SignalBuffer) Close() {
	for coin, ch := range sb.windows {
		err := ch.Close()
		if err != nil {
			log.Err(err).Str("coin", coin).Msg("error closing buffer")
		}
	}
}
