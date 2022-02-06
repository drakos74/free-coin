package ml

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/math"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/rs/zerolog/log"
)

type collector struct {
	store   storage.Persistence
	windows map[model.Key]*buffer.HistoryWindow
	state   map[model.Key]*state
	config  Config
}

type state struct {
	buffer *buffer.MultiBuffer
}

func newCollector(shard storage.Shard, _ *ff.Network, config Config) (*collector, error) {
	store, err := shard(Name)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		store = storage.NewVoidStorage()
	}

	windows := make(map[model.Key]*buffer.HistoryWindow)
	states := make(map[model.Key]*state)

	for k, cfg := range config.Segments {
		window := buffer.NewHistoryWindow(k.Duration, cfg.Stats.LookBack+cfg.Stats.LookAhead)
		windows[k] = &window
		states[k] = &state{
			buffer: buffer.NewMultiBuffer(cfg.Stats.LookBack),
		}
		log.Info().Str("key", k.ToString()).Msg("init collector")
		if err != nil {
			log.Error().Err(err).Str("k", fmt.Sprintf("%+v", k)).Msg("could not reset collector")
		}
	}

	return &collector{
		store:   store,
		windows: windows,
		state:   states,
		config:  config,
	}, nil
}

type vector struct {
	prevIn  []float64
	prevOut []float64
	newIn   []float64
	xx      []float64
	yy      []float64
}

func (c *collector) push(trade *model.Trade) (states map[time.Duration]vector, hasUpdate bool) {
	ss := make(map[time.Duration]vector)
	for k, window := range c.windows {
		if k.Match(trade.Coin) {
			tracker := c.state[k]
			if stateVector, ok := c.vector(window, tracker, trade, k); ok {
				ss[k.Duration] = stateVector
			}
		}
	}
	if len(ss) > 0 {
		return ss, true
	}
	return nil, false
}

func (c *collector) vector(window *buffer.HistoryWindow, tracker *state, trade *model.Trade, key model.Key) (vector, bool) {
	if b, ok := window.Push(trade.Time, trade.Price); ok {
		metrics.Observer.IncrementEvents(string(trade.Coin), key.Hash(), "window", Name)
		xx, yy, err := window.Extract(0, func(b buffer.TimeWindowView) float64 {
			return 100 * b.Ratio
		})
		if err != nil {
			// TODO : log at debug maybe ?
			return vector{}, false
		}
		inp, err := fit(xx, yy, 0, 1, 2)
		prev := tracker.buffer.Last()
		ratio := b.Values().Stats()[0].Ratio()
		count := b.Values().Stats()[0].Count()
		inp = append(inp, float64(count))
		next := make([]float64, 3)
		threshold := c.config.Segments[key].Stats.Gap
		if ratio > threshold {
			next[0] = 1
		} else if ratio < -1*threshold {
			next[2] = 1
		} else {
			next[1] = 1
		}
		if _, ok := tracker.buffer.Push(inp); ok {
			return vector{
				prevIn:  prev,
				prevOut: next,
				newIn:   inp,
				yy:      yy,
				xx:      xx,
			}, true
		}
	}
	return vector{}, false
}

func fit(xx, yy []float64, degree ...int) ([]float64, error) {
	p := make([]float64, len(degree))
	//fmt.Printf("xx = %+v\n", xx)
	//fmt.Printf("yy = %+v\n", yy)
	for i, d := range degree {
		if len(yy) < d+1 {
			return nil, fmt.Errorf("not enough buckets (%d out of %d) to apply polynomial regression for %d",
				len(yy),
				d+1,
				d)
		}
		f, err := math.Fit(xx, yy, d)
		if err != nil {
			return nil, fmt.Errorf("could not fit for degree '%d': %w", d, err)
		}
		if len(f) <= 0 {
			return nil, fmt.Errorf("could not fit for degree '%d': %v", d, f)
		}
		// round to reasonable number
		//fmt.Printf("i = %+v\n", i)
		//fmt.Printf("f = %+v\n", f)
		//fmt.Printf("f[d] = %+v\n", f[d])
		p[i] = f[d]
	}
	return p, nil
}
