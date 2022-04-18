package ml

import (
	"fmt"
	"math"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
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
	windows map[model.Key]*buffer.BatchWindow
	state   map[model.Key]*state
	vectors chan mlmodel.Vector
	config  *mlmodel.Config
}

type state struct {
	buffer *buffer.MultiBuffer
}

func newCollector(dim int, shard storage.Shard, _ *ff.Network, config *mlmodel.Config) (*collector, error) {
	store, err := shard(Name)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		store = storage.NewVoidStorage()
	}

	windows := make(map[model.Key]*buffer.BatchWindow)
	states := make(map[model.Key]*state)

	col := &collector{
		store:   store,
		config:  config,
		vectors: make(chan mlmodel.Vector),
	}

	for k, cfg := range config.Segments {
		bw, trades := buffer.NewBatchWindow(string(k.Coin), dim, k.Duration, cfg.Stats.LookBack+cfg.Stats.LookAhead)
		windows[k] = bw
		states[k] = &state{
			buffer: buffer.NewMultiBuffer(cfg.Stats.LookBack),
		}
		log.Info().Str("Key", k.ToString()).Msg("init collector")
		go col.process(k, trades)
	}

	col.windows = windows
	col.state = states

	return col, nil
}

func (c *collector) process(key model.Key, batch <-chan []buffer.StatsMessage) {
	metrics.Observer.IncrementEvents(string(key.Coin), key.Hash(), "window", Name)
	for messages := range batch {
		xx := make([]float64, 0)
		yy := make([]float64, 0)
		dv := make([]float64, 0)
		dp := make([]float64, 0)
		t0 := 0.0
		last := buffer.StatsMessage{}
		first := true
		for _, bucket := range messages {
			if bucket.OK && bucket.Time.Unix() > 0 {
				if first {
					t0 = float64(bucket.Time.Unix()) / bucket.Duration.Seconds()
					first = false
				}
				x := float64(bucket.Time.Unix())/bucket.Duration.Seconds() - t0
				xx = append(xx, x)

				y := math.Round(100 * bucket.Stats[0].Ratio())
				yy = append(yy, y)
				dv = append(dv, math.Round(10000*bucket.Stats[2].Avg()))
				dp = append(dp, math.Round(10000*bucket.Stats[3].Avg()))
				last = bucket
			}
		}
		inp, err := fit(xx, yy, 1, 2)
		if err != nil || !last.OK {
			log.Debug().Err(err).
				Str("Key", fmt.Sprintf("%+v", key)).
				Str("x", fmt.Sprintf("%+v", xx)).
				Str("y", fmt.Sprintf("%+v", yy)).
				Msg("could not fit")
			continue
		}
		ddv, _ := fit(xx, dv, 2)
		inp = append(inp, ddv...)
		ddp, _ := fit(xx, dp, 1)
		inp = append(inp, ddp...)
		tracker := c.state[key]
		prev := tracker.buffer.Last()
		ratio := last.Stats[0].Ratio()
		count := last.Stats[0].Count()
		price := last.Stats[0].Avg()
		volume := last.Stats[1].Avg()
		std := 0.0
		ema := 0.0
		if price != 0 {
			std = last.Stats[0].StDev() / math.Sqrt(price)
			ema = last.Stats[0].EMA() / price
		}
		inp = append(inp, float64(count)/last.Duration.Seconds(), std, ema)
		next := make([]float64, 3)
		threshold := c.config.Segments[key].Stats.Gap
		if ratio > threshold {
			next[0] = 1
		} else if ratio < -1*threshold {
			next[2] = 1
		} else {
			next[1] = 1
		}
		if _, ok := tracker.buffer.Push(inp...); ok {
			v := mlmodel.Vector{
				Meta: mlmodel.Meta{
					Key: key,
					Tick: model.Tick{
						Level: model.Level{
							Price:  price,
							Volume: volume,
						},
						Time: last.Time.Add(last.Duration),
					},
				},
				PrevIn:  prev,
				PrevOut: next,
				NewIn:   inp,
			}
			c.vectors <- v
		}
	}
}

func (c *collector) push(trade *model.TradeSignal) {
	for k, window := range c.windows {
		if k.Match(trade.Coin) {
			window.Push(trade.Tick.Time, trade.Tick.Price, trade.Tick.Volume, trade.Tick.Move.Velocity, trade.Tick.Move.Momentum)
		}
	}
}

func fit(xx, yy []float64, degree ...int) ([]float64, error) {
	p := make([]float64, len(degree))
	//fmt.Printf("XX = %+v\n", XX)
	//fmt.Printf("YY = %+v\n", YY)
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
