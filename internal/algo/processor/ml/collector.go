package ml

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/storage/file/json"

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
	history storage.Persistence
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

	// keep the events for history purposes
	var history storage.Persistence
	if config.Buffer.History {
		history = json.NewJsonBlob("ml", "history", false)
	}

	windows := make(map[model.Key]*buffer.BatchWindow)
	states := make(map[model.Key]*state)

	col := &collector{
		store:   store,
		history: history,
		config:  config,
		vectors: make(chan mlmodel.Vector),
	}

	for k, cfg := range config.Segments {
		bw, trades := buffer.NewBatchWindow(string(k.Coin), dim, k.Duration, cfg.Stats.LookBack+cfg.Stats.LookAhead)
		bw.WithEcho()
		windows[k] = bw
		states[k] = &state{
			buffer: buffer.NewMultiBuffer(cfg.Stats.LookBack),
		}
		log.Info().Str("Index", k.ToString()).Msg("init collector")
		go col.process(k, trades)
	}

	col.windows = windows
	col.state = states

	return col, nil
}

func (c *collector) push(trade *model.TradeSignal) {
	for k, window := range c.windows {
		if k.Match(trade.Coin) {
			window.Push(trade.Tick.Time, trade.Tick.Price, trade.Tick.Volume, trade.Tick.Move.Velocity, trade.Tick.Move.Momentum)
		}
	}
}

func (c *collector) process(key model.Key, batch <-chan []buffer.StatsMessage) {
	metrics.Observer.IncrementEvents(string(key.Coin), key.Hash(), "window", Name)
	var history [][]buffer.StatsMessage
	k := storage.Key{
		Pair:  string(key.Coin),
		Label: fmt.Sprintf("%+v", key.Duration.Minutes()),
	}
	if c.history != nil {
		if err := c.history.Load(k, &history); err != nil {
			log.Warn().Err(err).
				Str("pair", k.Pair).
				Str("label", k.Label).
				Int64("hash", k.Hash).
				Msg("no previous data for history")
		}
	}

	for messages := range batch {
		// keep track of history
		if c.history != nil {
			// append new data to data
			history = append(history, messages)
			// store into the list
			if err := c.history.Store(k, history); err != nil {
				log.Error().
					Err(err).
					Str("key", fmt.Sprintf("%+v", k)).
					Int("data", len(history)).
					Msg("could not store data set for history")
			}
		}

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

				y := 100 * bucket.Stats[0].Ratio()
				yy = append(yy, y)
				dv = append(dv, 100*bucket.Stats[2].Avg())
				dp = append(dp, 100*bucket.Stats[3].Avg())
				last = bucket
			} else {
				log.
					Error().
					Str("coin", string(key.Coin)).
					Str("bucket", fmt.Sprintf("%+v", bucket)).
					Msg("missed bucket")
			}
		}

		if !last.OK {
			log.Debug().
				Str("coin", string(key.Coin)).
				Str("Index", fmt.Sprintf("%+v", key)).
				Int("batch-size", len(batch)).
				Msg("no batch")
			continue
		}

		ratio := last.Stats[0].Ratio()
		count := last.Stats[0].Count()
		price := last.Stats[0].Avg()
		std := last.Stats[0].StDev()
		ema := last.Stats[0].EMA()
		volume := last.Stats[1].Avg()
		min, max := last.Stats[0].Range()
		// add the price fit polynomials
		// [ linear-price , quadratic-price ... ]
		inp, err := fit(xx, yy, 1, 2)
		if err != nil && len(yy) > 2 && len(xx) == len(yy) {
			log.Error().
				Err(err).
				Time("bucket-time", last.Time).
				Str("coin", string(key.Coin)).
				Str("xx", fmt.Sprintf("%+v", xx)).
				Str("yy", fmt.Sprintf("%+v", yy)).
				Msg("could not fit price")
		}
		// add the velocity fit polynomials
		ddv, err := fit(xx, dv, 2)
		if err != nil && len(dv) > 2 && len(xx) == len(dv) {
			log.Error().
				Err(err).
				Time("bucket-time", last.Time).
				Str("coin", string(key.Coin)).
				Str("dv", fmt.Sprintf("%+v", dv)).
				Str("ddv", fmt.Sprintf("%+v", ddv)).
				Msg("could not fit velocity")
		}
		// [ (1) (2) quadratic velocity ... ]
		inp = append(inp, ddv...)
		// add the momentum fit polynomials
		ddp, err := fit(xx, dp, 1, 2)
		if err != nil && len(dp) > 2 && len(xx) == len(dp) {
			log.Error().
				Err(err).
				Time("bucket-time", last.Time).
				Str("coin", string(key.Coin)).
				Str("dp", fmt.Sprintf("%+v", dp)).
				Str("ddp", fmt.Sprintf("%+v", ddp)).
				Msg("could not fit momentum")
		}
		// [ (1) (2) (3) linear-momentum quadratic-momentum ... ]
		inp = append(inp, ddp...)
		// add statistical data
		// [ (1) (2)  (3) (4) (5) frequency-of-events standard-deviation ema ]
		inp = append(inp, float64(count)/last.Duration.Seconds(), std, ema)
		// build the next state
		tracker := c.state[key]
		prev := tracker.buffer.Last()
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
						Range: model.Range{
							From: model.Event{
								Price: min,
								Time:  last.Time,
							},
							To: model.Event{
								Price: max,
								Time:  last.Time.Add(last.Duration),
							},
						},
						Move: model.Move{
							Velocity: last.Stats[2].Avg(),
							Momentum: last.Stats[3].Avg(),
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
