package buffer

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStats_Push(t *testing.T) {

	l := 1001

	type test struct {
		transform func(i int) float64
		avg       float64
		count     int
		diff      float64
		stDev     float64
		variance  float64
		sum       float64
		ema       float64
	}

	tests := map[string]test{
		"monotonically-increasing-+": {
			transform: func(i int) float64 {
				return float64(i)
			},
			avg:      float64(l / 2),
			count:    l,
			sum:      float64(l) * 500,
			diff:     float64(l) - 1,
			stDev:    289,
			variance: 83500,
			// note : ema is higher than the average as th sequence grows
			ema: 667,
		},
		"monotonically-increasing-0": {
			transform: func(i int) float64 {
				return float64(-1*l/2) + float64(i)
			},
			avg:   0,
			count: l,
			sum:   0,
			// NOTE : these are the same as the one above
			diff:     float64(l) - 1,
			stDev:    289,
			variance: 83500,
			// note : ema is higher than the average , as the sequence grows.
			ema: 167,
		},
		"monotonically-increasing--": {
			transform: func(i int) float64 {
				return (-1*float64(l) + 1) + float64(i)
			},
			avg:   -1 * float64(l/2),
			count: l,
			sum:   -1 * float64(l) * 500,
			// NOTE : these are the same as the one above
			diff:     float64(l) - 1,
			stDev:    289,
			variance: 83500,
			// note : ema is higher than the average , as the sequence grows.
			ema: -333,
		},
		"monotonically-decreasing-+": {
			transform: func(i int) float64 {
				return float64(l) - float64(i)
			},
			avg:   float64((l + 1) / 2),
			count: l,
			sum:   float64(l) * 501,
			diff:  -1 * (float64(l) - 1),
			// NOTE : these are the same as for the increasing case
			stDev:    289,
			variance: 83500,
			// note : ema is lower than the average , as the sequence shrinks.
			ema: 334,
		},
		"monotonically-decreasing-0": {
			transform: func(i int) float64 {
				return float64(l/2) - float64(i)
			},
			avg:   0,
			count: l,
			sum:   0,
			diff:  -1 * (float64(l) - 1),
			// NOTE : these are the same as for the increasing case
			stDev:    289,
			variance: 83500,
			// note : ema is lower than the average , as the sequence shrinks.
			ema: -167,
		},
		"monotonically-decreasing--": {
			transform: func(i int) float64 {
				return -1 * float64(i)
			},
			avg:   -1 * float64(l/2),
			count: l,
			sum:   -1 * float64(l) * 500,
			diff:  -1 * (float64(l) - 1),
			// NOTE : these are the same as the one above
			stDev:    289,
			variance: 83500,
			// note : ema is lower than the average , as the sequence shrinks.
			ema: -667,
		},
		"abs-+": {
			transform: func(i int) float64 {
				return math.Abs(-1*float64(l/2) + float64(i))
			},
			avg:   float64(l / 4),
			count: l,
			sum:   250500,
			diff:  0,
			// NOTE : these are half of the monotonical case
			stDev:    289 / 2,
			variance: 83500 / 4,
			// note : ema is the same as th average, as it is a symmetrical distribution
			ema: 250,
		},
		"abs--": {
			transform: func(i int) float64 {
				return -1 * math.Abs(-1*float64(l/2)+float64(i))
			},
			avg:   -1 * float64(l/4),
			count: l,
			sum:   -250500,
			diff:  0,
			// NOTE : these are half of the monotonical case
			stDev:    289 / 2,
			variance: 83500 / 4,
			// note : ema is the same as th average, as it is a symmetrical distribution
			ema: -250,
		},
		"sin": {
			transform: func(i int) float64 {
				return float64(l) * math.Sin(float64(i))
			},
			avg:      1, // very close to the expected '0' anyway
			count:    l,
			sum:      815,
			diff:     828,
			stDev:    708,    // NOTE : how much larger this is now
			variance: 500692, // NOTE : how much larger this is now
			// note : ema is very close to the average, as it is a symmetrical distribution
			ema: 0,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			stats := NewStats()
			for i := 0; i < l; i++ {
				v := tt.transform(i)
				stats.Push(v)
			}
			assert.Equal(t, tt.avg, math.Round(stats.Avg()))
			assert.Equal(t, tt.count, stats.Count())
			assert.Equal(t, tt.sum, math.Round(stats.Sum()))
			assert.Equal(t, tt.diff, math.Round(stats.Diff()))
			assert.Equal(t, tt.stDev, math.Round(stats.StDev()))
			assert.Equal(t, tt.variance, math.Round(stats.Variance()))
			assert.Equal(t, tt.ema, math.Round(stats.EMA()))
		})
	}
}

func TestWindow(t *testing.T) {
	window := NewWindow(3, 1)
	c := 0
	for i := 0; i < 100; i++ {
		if _, _, ok := window.Push(int64(i), float64(i)); ok {
			c++
		}
	}
	assert.Equal(t, c, 33, fmt.Sprintf("expecting only 33 events, due to the size of the window"))
}
