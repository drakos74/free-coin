package buffer

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
						// the first bucket might not be the full Duration, due to the way we Index on time.
						assert.Equal(t, 1*time.Minute, d)
						stats := b.Values().Stats()[0]
						assert.Equal(t, int(tt.duration.Seconds()), stats.Count())
						// TODO : add more assertions on the numbers
					}

					current = b.Time
					doneFirst = true
				}
				// TODO : what would be the assertions for the 'else' here ?
			}
		})
	}
}

func TestIntervalWindow_Flush(t *testing.T) {

	iw, stats := NewIntervalWindow("", 3, time.Second)

	go func() {
		for i := 0; i < 1000; i++ {
			f := float64(i)
			iw.Push(time.Now(), f, 10*f, f/5)
			time.Sleep(10 * time.Millisecond)
		}
		iw.Close()
	}()

	c := 0
	for stat := range stats {
		fmt.Printf("stat-message = %+v\n", stat)
		c++
	}

	fmt.Printf("c = %+v\n", c)
	assert.True(t, 11 <= c && c <= 12)

}

func TestBatchWindow_Push(t *testing.T) {

	bw, stats := NewBatchWindow("", 1, time.Second, 5)

	go func() {
		for i := 0; i < 1000; i++ {
			f := float64(i)
			bw.Push(time.Now(), f)
			time.Sleep(10 * time.Millisecond)
		}
		bw.Close()
	}()

	c := 0
	for stat := range stats {
		fmt.Printf("stat-message = %+v\n", stat)
		c++
	}

	fmt.Printf("c = %+v\n", c)
	assert.True(t, 11 <= c && c <= 12)
}
