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

					fmt.Printf("b = %+v\n", b)
					fmt.Printf("d = %+v\n", d)
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
	e := 0
	for stat := range stats {
		if stat.OK {
			c++
			e += stat.Stats[0].Count()
		} else {
			assert.Fail(t, "no faulty message expected for this config")
		}

	}

	assert.True(t, 9 <= c && c <= 11, fmt.Sprintf("experiment set up of 1000 events at a rate of 1 per 10ms should last around 10s, whcih means around 10 events. We got %+v", c))
	assert.True(t, 900 <= e && e <= 1000, fmt.Sprintf("unexpected number of events : %v", e))
}

func TestIntervalWindow_NoEcho(t *testing.T) {

	window, stats := NewIntervalWindow("", 1, time.Second)

	go func() {
		for i := 0; i < 10; i++ {
			window.Push(time.Now(), float64(i))
			time.Sleep(1500 * time.Millisecond)
		}
		window.Close()
	}()

	c := 0
	e := 0
	v := 0
	for stat := range stats {
		if stat.OK {
			c++
			e += stat.Stats[0].Count()
		} else {
			v++
		}
	}

	assert.Equal(t, c, 10)
	assert.Equal(t, e, 10)
	assert.Equal(t, v, 4)
}

func TestIntervalWindow_Echo(t *testing.T) {

	window, stats := NewIntervalWindow("", 1, time.Second)
	window.WithEcho()

	go func() {
		for i := 0; i < 10; i++ {
			window.Push(time.Now(), float64(i))
			time.Sleep(1500 * time.Millisecond)
		}
		window.Close()
	}()

	c := 0
	e := 0
	for stat := range stats {
		if stat.OK {
			c++
			e += stat.Stats[0].Count()
		} else {
			assert.Fail(t, "no fault messages expected with this config")
		}
	}

	assert.Equal(t, c, 14)
	assert.Equal(t, e, 10)

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
	e := 0
	for stat := range stats {
		if stat[0].OK {
			c++
			e += stat[0].Stats[0].Count()
		} else {
			assert.Fail(t, "no fault messages expected with this config")
		}
	}

	assert.True(t, 9 <= c && c <= 11, fmt.Sprintf("experiment set up of 1000 events at a rate of 1 per 10ms should last around 10s, whcih means around 10 events. We got %+v", c))
	assert.True(t, 900 <= e && e <= 1000, fmt.Sprintf("unexpected number of events : %v", e))

}

func TestBatchWindow_NoEcho(t *testing.T) {

	bw, stats := NewBatchWindow("", 1, time.Second, 5)

	go func() {
		for i := 0; i < 10; i++ {
			f := float64(i)
			bw.Push(time.Now(), f)
			time.Sleep(1500 * time.Millisecond)
		}
		bw.Close()
	}()

	c := 0
	e := 0
	v := 0
	for stat := range stats {
		if stat[0].OK {
			c++
			e += stat[0].Stats[0].Count()
		} else {
			v++
		}
	}

	assert.Equal(t, c, 11)
	assert.Equal(t, e, 11)
	assert.Equal(t, v, 3)
}

func TestBatchWindow_Echo(t *testing.T) {

	bw, stats := NewBatchWindow("", 1, time.Second, 5)
	bw.WithEcho()

	go func() {
		for i := 0; i < 10; i++ {
			f := float64(i)
			bw.Push(time.Now(), f)
			time.Sleep(1500 * time.Millisecond)
		}
		bw.Close()
	}()

	c := 0
	e := 0
	for stat := range stats {
		if stat[0].OK {
			c++
			e += stat[0].Stats[0].Count()
		} else {
			assert.Fail(t, "no fault messages expected with this config")
		}
	}

	assert.True(t, 14 <= c && c <= 15, fmt.Sprintf("got %+v", c))
	assert.True(t, 11 <= e && e <= 12, fmt.Sprintf("got %+v", e))
}
