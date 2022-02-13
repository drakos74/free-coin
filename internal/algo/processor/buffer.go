package processor

import (
	"time"

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
func (tb *TradesBuffer) Push(trade *model.Trade) (*model.Trade, bool) {
	if _, ok := tb.windows[string(trade.Coin)]; !ok {
		tb.windows[string(trade.Coin)] = buffer.NewHistoryWindow(tb.duration, tb.size)
	}
	if bucket, ok := tb.windows[string(trade.Coin)].Push(trade.Time, trade.Price, trade.Volume); ok {
		t := model.SignedType(bucket.Values().Stats()[0].Ratio())
		trade = &model.Trade{
			Coin:   trade.Coin,
			Price:  bucket.Values().Stats()[0].Avg(),
			Type:   t,
			Volume: bucket.Values().Stats()[1].Avg(),
			Active: true,
			Live:   true,
			Time:   trade.Time,
		}
		return trade, true
	}
	return nil, false
}
