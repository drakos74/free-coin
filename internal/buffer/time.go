package buffer

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/math"
)

const (
	interval = time.Minute
)

// TimeWindowView is a time specific but static snapshot on top of a StatsCollector.
// It allows to retrieve buckets of Stats from a streaming data set.
// the bucket indexing is based on the time.
type TimeWindowView struct {
	Time    time.Time `json:"time"`
	Count   int       `json:"Count"`
	Value   float64   `json:"value"`
	EMADiff float64   `json:"ema_diff"`
	Diff    float64   `json:"diff"`
	Ratio   float64   `json:"ratio"`
	StdDev  float64   `json:"std"`
	Density int       `json:"density"`
}

// NewView creates a new time view from a time bucket.
func NewView(bucket TimeBucket, index int) TimeWindowView {
	avg := bucket.Values().Stats()[index].Avg()
	ema := bucket.Values().Stats()[index].EMA()
	diff := bucket.Values().Stats()[index].Diff()
	count := bucket.Values().Stats()[index].Count()
	ratio := bucket.Values().Stats()[index].Ratio()
	// Note: trying to make all values relative to the price
	// so that they have a unified meaning
	return TimeWindowView{
		Time:    bucket.Time,
		Count:   bucket.Size(),
		Value:   avg,
		Diff:    diff,
		EMADiff: 100 * (ema - avg) / avg,
		Ratio:   ratio,
		StdDev:  100 * bucket.Values().Stats()[index].StDev() / avg,
		Density: count,
	}
}

// TimeBucket is a wrapper for a bucket of a TimeWindow, that carries also the time Index.
type TimeBucket struct {
	Bucket
	Time time.Time
}

// TimeWindow is a Window indexed by the bucket time.
type TimeWindow struct {
	Index    int64   `json:"Index"`
	Duration int64   `json:"Duration"`
	window   *Window `json:"-"`
}

// NewTimeWindow creates a new TimeWindow with the given Duration.
func NewTimeWindow(duration time.Duration) TimeWindow {
	d := int64(duration.Seconds())
	return TimeWindow{
		Duration: d,
		window:   NewWindow(1),
	}
}

// Push adds an element to the time Window.
// It will return true, if the last addition caused a bucket to close.
func (tw TimeWindow) Push(t time.Time, v ...float64) (TimeBucket, bool) {

	// TODO : provide a inverse hash operation
	index := t.Unix() / tw.Duration

	index, bucket, closed := tw.window.Push(index, v...)

	if closed {
		tw.Index = index
		return TimeBucket{
			Bucket: bucket,
			Time:   time.Unix(bucket.index*tw.Duration, 0),
		}, true
	}

	return TimeBucket{}, false

}

// Next returns the next timestamp for the coming Window.
func (tw *TimeWindow) Next(iterations int64) time.Time {
	nextIndex := tw.Index + tw.Duration*(iterations+1)
	return time.Unix(nextIndex*int64(time.Second.Seconds()), 0)
}

// HistoryWindow keeps the last x buckets based on the Window interval given
type HistoryWindow struct {
	Window  TimeWindow `json:"Window"`
	buckets *Ring      `json:"-"`
}

// NewHistoryWindow creates a new history Window.
func NewHistoryWindow(duration time.Duration, size int) HistoryWindow {
	return HistoryWindow{
		Window:  NewTimeWindow(duration),
		buckets: NewRing(size),
	}
}

// Push adds an element to the given time Index.
// It will return true, if there was a new bucket completed at the last operation
func (h HistoryWindow) Push(t time.Time, v ...float64) (TimeBucket, bool) {
	if bucket, ok := h.Window.Push(t, v...); ok {
		h.buckets.Push(bucket)
		return bucket, true
	}
	return TimeBucket{}, false
}

// Get returns the transformed bucket value at the corresponding Index.
// TODO : change the signature to act like json.Decode etc... so that we control the appending of properties on our own
func (h HistoryWindow) Get(transform TimeBucketTransform) []interface{} {
	return h.buckets.Get(func(bucket interface{}) interface{} {
		// it's a history window , so we expect to have history buckets inside
		if b, ok := bucket.(TimeBucket); ok {
			return transform(b)
		}
		// TODO : this will introduce a nil element into the slice ... CAREFUL !!!
		return nil
	})
}

// Buffer evaluates the elements of the current buffer
func (h HistoryWindow) Buffer(index int, extract func(b TimeWindowView) float64) ([]float64, error) {
	yy := make([]float64, 0)
	buckets := h.buckets.Get(func(bucket interface{}) interface{} {
		if b, ok := bucket.(TimeBucket); ok {
			return NewView(b, index)
		}
		return bucket
	})

	for _, bucket := range buckets {
		if view, ok := bucket.(TimeWindowView); ok {
			y := extract(view)
			yy = append(yy, y)
		} else {
			return nil, fmt.Errorf("window does not contain timebuckets")
		}
	}

	//log.Debug().Str("xx", fmt.Sprintf("%+v", xx)).Str("yy", fmt.Sprintf("%+v", yy)).Msg("fit")

	return yy, nil
}

// Polynomial evaluates the polynomial regression
// for the given polynomial degree
// and based on the value extracted from the TimeBucket
// scaled at the corresponding time duration.
func (h HistoryWindow) Polynomial(index int, extract func(b TimeWindowView) float64, degree int) ([]float64, error) {
	xx := make([]float64, 0)
	yy := make([]float64, 0)
	t0 := 0.0
	buckets := h.buckets.Get(func(bucket interface{}) interface{} {
		if b, ok := bucket.(TimeBucket); ok {
			return NewView(b, index)
		}
		return bucket
	})

	if len(buckets) < degree+1 {
		return nil, fmt.Errorf("not enough buckets (%d out of %d) to apply polynomial regression for %d",
			len(buckets),
			degree+1,
			degree)
	}
	for i, bucket := range buckets {
		if view, ok := bucket.(TimeWindowView); ok {
			if i == 0 {
				t0 = float64(view.Time.Unix()) / interval.Seconds()
			}
			x := float64(view.Time.Unix())/interval.Seconds() - t0
			xx = append(xx, x)

			y := extract(view)
			yy = append(yy, y)
		} else {
			return nil, fmt.Errorf("window does not contain timebuckets")
		}
	}

	//log.Debug().Str("xx", fmt.Sprintf("%+v", xx)).Str("yy", fmt.Sprintf("%+v", yy)).Msg("fit")

	return math.Fit(xx, yy, degree)
}

// StatsWindow is a predefined TimeBucketTransform function that will gather the stats for the given dimensions.
func StatsWindow(dim int) TimeBucketTransform {
	return func(bucket TimeBucket) interface{} {
		views := make([]TimeWindowView, dim)
		for i := 0; i < dim; i++ {
			views[i] = NewView(bucket, i)
		}
		return views
	}
}

func WindowDensity() TimeBucketTransform {
	return func(bucket TimeBucket) interface{} {
		return bucket.Values().Stats()[0].Count()
	}
}
