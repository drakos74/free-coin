package processor

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

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
				log.Info().
					Timestamp().
					Str("ok", fmt.Sprintf("%+v", bucket.OK)).
					Msg("bucket=debug")
				if bucket.OK {
					min, max := bucket.Stats[0].Range()
					size := bucket.Stats[0].Count()
					signal := &model.TradeSignal{
						Coin: coin,
						Tick: model.Tick{
							Time:   bucket.Time.Add(bucket.Duration),
							Active: bucket.OK,
							Range: model.Range{
								From: model.Event{
									Price: min,
									Time:  bucket.Time,
								},
								To: model.Event{
									Price: max,
									Time:  bucket.Time.Add(bucket.Duration),
								},
							},
						},
						Meta: model.Meta{
							Time: bucket.Time,
							Live: bucket.OK,
							Size: size,
						},
					}
					signal.Tick.Level = model.Level{
						Price:  bucket.Stats[0].Avg(),
						Volume: bucket.Stats[1].Avg(),
					}
					signals <- signal
				}
			}
		}(trade.Coin, sb.trades)
	}
	sb.windows[coin].Push(trade.Tick.Time, trade.Tick.Price, trade.Tick.Volume)
}

func (sb *SignalBuffer) Close() {
	for coin, ch := range sb.windows {
		err := ch.Close()
		if err != nil {
			log.Err(err).Str("coin", coin).Msg("error closing buffer")
		}
	}
}
