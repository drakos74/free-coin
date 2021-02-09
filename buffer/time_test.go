package buffer

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestHistoryWindow_Push(t *testing.T) {

	type test struct {
		duration time.Duration
		size     int
	}

	tests := map[string]test{
		"1m-6": {
			duration: 1 * time.Minute,
			size:     6,
		},
		// TODO : add more test cases
	}

	for name, tt := range tests {

		t.Run(name, func(t *testing.T) {
			window := NewHistoryWindow(tt.duration, tt.size)
			now := time.Now()
			current := now
			var doneFirst bool
			for i := 0; i < 1000; i++ {
				if b, ok := window.Push(now.Add(time.Duration(i)*time.Second), float64(i)); ok {
					d := b.Time.Sub(current)

					if doneFirst {
						// the first bucket might not be the full duration, due to the way we index on time.
						assert.Equal(t, 1*time.Minute, d)
						stats := b.Values().Stats()[0]
						assert.Equal(t, int(tt.duration.Seconds()), stats.Count())
						// TODO : add more assertions on the numbers
					}

					current = b.Time
					doneFirst = true
				} else {

				}
			}
		})
	}
}
