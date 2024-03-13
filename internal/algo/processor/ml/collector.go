package ml

import (
	"fmt"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

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

// Collector collects trade stats based on the collector functions
// it groups previous trade stats, so we can join the previous and next stats
// and can effectively train a model
type Collector struct {
	store   storage.Persistence
	history storage.Persistence
	tracker map[model.Key]*buffer.MultiBuffer
	vectors chan mlmodel.Vector
	config  mlmodel.Config
	in      Collect
	out     Collect
}

func NewCollector(shard storage.Shard, config mlmodel.Config, in, out Collect) (*Collector, error) {
	store, err := shard(Name)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		store = storage.NewVoidStorage()
	}

	tracker := make(map[model.Key]*buffer.MultiBuffer)

	col := &Collector{
		store:   store,
		config:  config,
		vectors: make(chan mlmodel.Vector),
		in:      in,
		out:     out,
	}

	for k, cfg := range config.Segments {
		tracker[k] = buffer.NewMultiBuffer(cfg.Stats.LookBack + cfg.Stats.LookAhead)
		log.Info().Str("Index", k.ToString()).Msg("init collector")
	}

	col.tracker = tracker

	return col, nil
}

func (c *Collector) push(trade *model.TradeSignal) {
	for k, track := range c.tracker {
		if k.Match(trade.Coin) {
			x := c.in(trade)
			prev := track.Last()

			if _, fill := track.Push(x...); fill {
				// format to the observed output vector
				next := c.out(trade)
				vector := mlmodel.Vector{
					Meta: mlmodel.Meta{
						Key:  k,
						Tick: trade.Tick,
					},
					PrevIn:  prev,
					PrevOut: next,
					NewIn:   x,
				}
				c.vectors <- vector
			}
		}
	}
}

type Collect func(trade *model.TradeSignal) []float64

var Trend Collect = func(trade *model.TradeSignal) []float64 {
	// pick the right attributes for the ml input vector
	trend := 100 * trade.Tick.StatsData.Trend.Price / trade.Tick.Price
	return []float64{trend, trade.Tick.Price}
}

var CollectStats Collect = func(trade *model.TradeSignal) []float64 {
	// pick the right attributes for the ml input vector
	trend := 100 * trade.Tick.StatsData.Trend.Price / trade.Tick.Price
	std := trade.Tick.StatsData.Std.Price / trade.Tick.Price
	volume := trade.Tick.StatsData.Std.Volume / trade.Tick.Volume
	size := float64(trade.Meta.Size)
	buyEvents := trade.Tick.StatsData.Buy.Count / size
	sellEvents := trade.Tick.StatsData.Sell.Count / size
	buyVolume := trade.Tick.StatsData.Buy.Volume / trade.Tick.Volume
	sellVolume := trade.Tick.StatsData.Sell.Volume / trade.Tick.Volume
	return []float64{trend, std, volume, buyVolume, buyEvents, sellVolume, sellEvents, size, trade.Tick.Price}
}

var SplitOnTrend = func(gap float64) Collect {
	threshold := gap
	return func(trade *model.TradeSignal) []float64 {
		trend := trade.Tick.StatsData.Trend.Price / trade.Tick.Price
		y := make([]float64, 3)
		if trend > threshold {
			y[0] = 1
		} else if trend < -1*threshold {
			y[2] = 1
		} else {
			y[1] = 1
		}
		return y
	}
}
