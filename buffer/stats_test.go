package buffer

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestSet_Push(t *testing.T) {

	set := NewStats()

	for i := 0; i < 100; i++ {
		set.Push(float64(i))
		fmt.Println(fmt.Sprintf("set.count = %+v", set.count))
		avg := set.Avg()
		diff := set.Diff()
		variance := set.Variance()
		stdev := set.StDev()
		fmt.Println(fmt.Sprintf("avg = %+v", avg))
		fmt.Println(fmt.Sprintf("diff = %+v", diff))
		fmt.Println(fmt.Sprintf("variance = %+v", variance))
		fmt.Println(fmt.Sprintf("stdev = %+v", stdev))
	}

}

func TestWindow(t *testing.T) {

	window := NewWindow(10)

	for i := 0; i < 100; i++ {
		window.Push(int64(i), float64(i))
		fmt.Println(fmt.Sprintf("window = %+v", window))
	}

}

func TestTimeWindow(t *testing.T) {

	window := NewTimeWindow(time.Second * 10)

	now := time.Now()

	for i := 0; i < 100; i++ {
		t, bucket, ok := window.Push(now.Add(time.Second*time.Duration(i)), float64(i))
		fmt.Println(fmt.Sprintf("t = %+v", t))
		fmt.Println(fmt.Sprintf("bucket = %+v", bucket))
		fmt.Println(fmt.Sprintf("ok = %+v", ok))

		fmt.Println(fmt.Sprintf("time-window = %+v", window))
		fmt.Println(fmt.Sprintf("window = %+v", *window.window))

	}

}

func TestHistoryWindow_Increasing(t *testing.T) {

	type test struct {
		interval int
		size     int
		op       func(i int) float64
	}

	tests := map[string]test{
		"increasing": {
			interval: 5,
			size:     10,
			op: func(i int) float64 {
				return float64(i)
			},
		},
		"decreasing": {
			interval: 5,
			size:     10,
			op: func(i int) float64 {
				return 100 - float64(i)
			},
		},
		"sine": {
			interval: 5,
			size:     10,
			op: func(i int) float64 {
				return math.Sin(0.1 * float64(i))
			},
		},
		"mv": {
			interval: 5,
			size:     10,
			op: func(i int) float64 {
				return 10 + 10*math.Sin(0.1*float64(i))
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// we should group at every tt.interval second mark
			// and keep tt.size buckets of historical data e.g. [interval*size] seconds
			window := NewHistoryWindow(time.Second*time.Duration(tt.interval), tt.size)

			now := time.Now()

			for i := 0; i < 100; i++ {

				value := tt.op(i)
				fmt.Println(fmt.Sprintf("value = %+v", value))
				// we emulate as if each event is 1 second(s) apart
				if t, bucket, ok := window.Push(now.Add(time.Second*time.Duration(i)), value); ok {
					println(fmt.Sprintf("t = %+v", t))
					fmt.Println(fmt.Sprintf("i = %+v", i))
					avg := bucket.Values().Stats()[0].Avg()
					diff := bucket.Values().Stats()[0].Diff()
					std := bucket.Values().Stats()[0].StDev()
					size := bucket.Size()
					fmt.Println(fmt.Sprintf("diff/avg = %+v", diff/avg))
					fmt.Println(fmt.Sprintf("avg = %+v", avg))
					fmt.Println(fmt.Sprintf("diff = %+v", diff))
					fmt.Println(fmt.Sprintf("std = %+v", std))
					fmt.Println(fmt.Sprintf("size = %+v", size))
				}

				values := window.Get(func(bucket *Bucket) interface{} {
					return bucket.Values().Stats()[0].Avg()
				})

				fmt.Println(fmt.Sprintf("values = %+v", values))

				// get last bucket and make some assertions

			}
		})
	}

}
