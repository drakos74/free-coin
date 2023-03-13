package ml

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestCollector_Process(t *testing.T) {

	key := model.Key{
		Coin:     "TEST",
		Duration: 5 * time.Second,
	}
	col, err := newCollector(4, storage.MockShard(), nil, &mlmodel.Config{
		Segments: mlmodel.SegmentConfig{
			key: mlmodel.Segments{
				Stats: mlmodel.Stats{
					LookBack:  3,
					LookAhead: 1,
					Gap:       0.5,
				},
			},
		},
	})
	assert.NoError(t, err)

	go func() {
		for v := range col.vectors {
			fmt.Printf("v = %+v\n", v)
		}
	}()

	var c int
	enrich := processor.Deriv()
	for i := 0; i < 100; i++ {
		now := time.Now()
		col.push(enrich(&model.TradeSignal{
			Coin: "TEST",
			Meta: model.Meta{
				Time: now,
				Live: true,
			},
			Tick: model.Tick{
				Level: model.Level{
					Price:  sin(i, 10),
					Volume: 1,
				},
				Time:   now,
				Active: true,
			},
		}))
		c++
		time.Sleep(1000 * time.Millisecond)
	}

	fmt.Printf("c = %+v\n", c)

}

func TestCollector_Process_MissingEvent(t *testing.T) {

	key := model.Key{
		Coin:     "TEST",
		Duration: 5 * time.Second,
	}
	col, err := newCollector(4, storage.MockShard(), nil, &mlmodel.Config{
		Segments: mlmodel.SegmentConfig{
			key: mlmodel.Segments{
				Stats: mlmodel.Stats{
					LookBack:  3,
					LookAhead: 1,
					Gap:       0.5,
				},
			},
		},
	})
	assert.NoError(t, err)

	go func() {
		for v := range col.vectors {
			fmt.Printf("v = %+v\n", v)
		}
	}()

	var c int
	enrich := processor.Deriv()
	for i := 0; i < 100; i++ {
		now := time.Now()
		if i >= 10 && i <= 20 {
			// skip events
		} else {
			col.push(enrich(&model.TradeSignal{
				Coin: "TEST",
				Meta: model.Meta{
					Time: now,
					Live: true,
				},
				Tick: model.Tick{
					Level: model.Level{
						Price:  sin(i, 10),
						Volume: 1,
					},
					Time:   now,
					Active: true,
				},
			}))
			c++
		}
		time.Sleep(1000 * time.Millisecond)
	}

	fmt.Printf("c = %+v\n", c)

}

func TestCollector_ProcessRaw(t *testing.T) {

	config := &mlmodel.Config{
		Segments: mlmodel.SegmentConfig{
			model.Key{}: mlmodel.Segments{
				Stats: mlmodel.Stats{
					LookBack:  3,
					LookAhead: 1,
					Gap:       0.05,
				},
			},
		},
		Position: mlmodel.Position{},
		Option:   mlmodel.Option{},
		Buffer:   mlmodel.Buffer{},
	}

	col := collector{
		vectors: make(chan mlmodel.Vector),
		state: map[model.Key]*state{
			model.Key{}: {buffer: buffer.NewMultiBuffer(3)},
		},
		config: config,
	}

	key := model.Key{}

	stats := make(chan []buffer.StatsMessage)
	go col.process(key, stats)

	go func() {
		for v := range col.vectors {
			fmt.Printf("v = %+v\n", v)
		}
	}()

	var c int
	now := time.Now()
	for i := 0; i < 1000; i += 12 {
		stats <- []buffer.StatsMessage{
			newStatsMessage(now, i, sin(i, 100), sin(i+1, 100), sin(i+2, 100)),
			newStatsMessage(now, i+3, sin(i+3, 100), sin(i+4, 100), sin(i+5, 100)),
			newStatsMessage(now, i+6, sin(i+6, 100), sin(i+7, 100), sin(i+8, 100)),
			newStatsMessage(now, i+9, sin(i+9, 100), sin(i+10, 100), sin(i+11, 100)),
		}
		c++
		time.Sleep(10 * time.Millisecond)
	}

	fmt.Printf("c = %+v\n", c)

}

func sin(i int, granularity int) float64 {
	return 1000 + 100*math.Sin(float64(i)/float64(granularity))
}

func newStatsMessage(now time.Time, i int, first, f, last float64) buffer.StatsMessage {
	return buffer.StatsMessage{
		OK:       true,
		Time:     now.Add(time.Duration(i) * time.Minute),
		ID:       fmt.Sprintf("%+v", i),
		Duration: 1 * time.Second,
		Dim:      4,
		Stats: []buffer.Stats{
			buffer.MockStats(1, f, first, last),
			buffer.MockStats(1, f, first, last),
			buffer.MockStats(1, f, first, last),
			buffer.MockStats(1, f, first, last),
		},
	}
}
