package ml

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/metrics"

	"github.com/drakos74/go-ex-machina/xmachina/net/ff"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

type collector struct {
	store   storage.Persistence
	windows map[model.Coin]map[time.Duration]*buffer.HistoryWindow
	state   map[model.Coin]map[time.Duration]*state
	config  Config
}

type state struct {
	buffer *buffer.MultiBuffer
}

func newCollector(shard storage.Shard, _ *ff.Network, segments Config) (*collector, error) {
	store, err := shard(Name)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		store = storage.NewVoidStorage()
	}

	windows := make(map[model.Coin]map[time.Duration]*buffer.HistoryWindow)
	states := make(map[model.Coin]map[time.Duration]*state)

	for c, cfg := range segments {
		if _, ok := windows[c]; !ok {
			windows[c] = make(map[time.Duration]*buffer.HistoryWindow, 0)
			states[c] = make(map[time.Duration]*state, 0)
		}
		for d, segment := range cfg {
			window := buffer.NewHistoryWindow(d, segment.LookBack+segment.LookAhead)
			windows[c][d] = &window
			states[c][d] = &state{
				buffer: buffer.NewMultiBuffer(segment.LookBack),
			}
		}
		fmt.Printf("cfg = %+v\n", cfg)
	}

	return &collector{
		store:   store,
		windows: windows,
		state:   states,
		config:  segments,
	}, nil
}

type vector struct {
	prevIn  []float64
	prevOut []float64
	newIn   []float64
}

func (c *collector) push(trade *model.Trade) (states map[time.Duration]vector, hasUpdate bool) {
	coin := trade.Coin
	ss := make(map[time.Duration]vector)
	if ww, ok := c.windows[coin]; ok {
		tracker := c.state[coin]
		for d, window := range ww {
			if stateVector, ok := c.vector(window, tracker[d], trade, d); ok {
				ss[d] = stateVector
			}
		}
	}
	if len(ss) > 0 {
		return ss, true
	}
	return nil, false
}

func (c *collector) vector(window *buffer.HistoryWindow, tracker *state, trade *model.Trade, d time.Duration) (vector, bool) {
	if b, ok := window.Push(trade.Time, trade.Price); ok {
		metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "window", Name)
		p1, _ := window.Polynomial(0, func(b buffer.TimeWindowView) float64 {
			return b.Ratio
		}, 1)
		p2, _ := window.Polynomial(0, func(b buffer.TimeWindowView) float64 {
			return b.Ratio
		}, 2)
		p3, _ := window.Polynomial(0, func(b buffer.TimeWindowView) float64 {
			return b.Ratio
		}, 3)
		if len(p1) > 0 && len(p2) > 0 && len(p3) > 0 {
			prev := tracker.buffer.Last()
			ratio := b.Values().Stats()[0].Ratio()
			next := make([]float64, 3)
			threshold := c.config[trade.Coin][d].Threshold
			if ratio > threshold {
				next[0] = 1
			} else if ratio < -1*threshold {
				next[2] = 1
			} else {
				next[1] = 1
			}
			current := []float64{p1[1], p2[2], p3[3]}
			if _, ok := tracker.buffer.Push(current); ok {
				return vector{
					prevIn:  prev,
					prevOut: next,
					newIn:   current,
				}, true
			}
		} else {
			log.Warn().
				Floats64("p1", p1).
				Floats64("p2", p2).
				Floats64("p2", p2).
				Msg("not enough data to ingest")
		}
	}
	return vector{}, false
}
