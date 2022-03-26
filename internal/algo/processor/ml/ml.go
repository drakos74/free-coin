package ml

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/buffer"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/rs/zerolog/log"
)

type collector struct {
	store   storage.Persistence
	windows map[model.Key]buffer.BatchWindow
	state   map[model.Key]*state
	vectors chan vector
	config  *Config
}

type state struct {
	buffer *buffer.MultiBuffer
}

func newCollector(dim int64, shard storage.Shard, _ *ff.Network, config *Config) (*collector, error) {
	store, err := shard(Name)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		store = storage.NewVoidStorage()
	}

	windows := make(map[model.Key]buffer.BatchWindow)
	states := make(map[model.Key]*state)

	col := &collector{
		store:   store,
		config:  config,
		vectors: make(chan vector),
	}

	for k, cfg := range config.Segments {
		bw, trades := buffer.NewBatchWindow(dim, k.Duration, cfg.Stats.LookBack+cfg.Stats.LookAhead)
		windows[k] = bw
		states[k] = &state{
			buffer: buffer.NewMultiBuffer(cfg.Stats.LookBack),
		}
		log.Info().Str("key", k.ToString()).Msg("init collector")
		go col.process(k, trades)
	}

	col.windows = windows
	col.state = states

	return col, nil
}

type meta struct {
	coin     model.Coin
	duration time.Duration
	tick     model.Tick
}

type vector struct {
	meta    meta
	prevIn  []float64
	prevOut []float64
	newIn   []float64
	xx      []float64
	yy      []float64
}

func (c *collector) process(key model.Key, batch <-chan []buffer.StatsMessage) {
	metrics.Observer.IncrementEvents(string(key.Coin), key.Hash(), "window", Name)
	for messages := range batch {
		xx := make([]float64, 0)
		yy := make([]float64, 0)
		t0 := 0.0
		last := messages[len(messages)-1]
		for i, bucket := range messages {
			if i == 0 {
				t0 = float64(bucket.Time.Unix()) / bucket.Duration.Seconds()
			}
			x := float64(bucket.Time.Unix())/bucket.Duration.Seconds() - t0
			xx = append(xx, x)

			y := 100 * bucket.Stats[0].Ratio()
			yy = append(yy, y)
		}
		inp, err := fit(xx, yy, 0, 1, 2)
		if err != nil {
			log.Warn().Err(err).
				Str("key", fmt.Sprintf("%+v", key)).
				Str("x", fmt.Sprintf("%+v", xx)).
				Str("y", fmt.Sprintf("%+v", yy)).
				Msg("could not fit")
			continue
		}
		tracker := c.state[key]
		prev := tracker.buffer.Last()
		ratio := last.Stats[0].Ratio()
		count := last.Stats[0].Count()
		price := last.Stats[0].Avg()
		volume := last.Stats[1].Avg()
		value := price * volume
		std := last.Stats[0].StDev()
		inp = append(inp, float64(count), value, std)
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
			v := vector{
				meta: meta{
					coin:     key.Coin,
					duration: key.Duration,
					tick: model.Tick{
						Level: model.Level{
							Price:  price,
							Volume: volume,
						},
						Time: last.Time.Add(last.Duration),
					},
				},
				prevIn:  prev,
				prevOut: next,
				newIn:   inp,
				yy:      yy,
				xx:      xx,
			}
			c.vectors <- v
		}
	}
}

func (c *collector) push(trade *model.TradeSignal) {
	for k, window := range c.windows {
		if k.Match(trade.Coin) {
			window.Push(trade.Meta.Time, trade.Tick.Price, trade.Tick.Volume)
		}
	}
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
		f, err := coinmath.Fit(xx, yy, d)
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
