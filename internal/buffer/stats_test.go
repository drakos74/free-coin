package buffer

import (
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
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
		})
	}

}