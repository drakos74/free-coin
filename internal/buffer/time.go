package buffer

import "time"

// TimeWindowView is a time specific but static snapshot on top of a StatsCollector.
// It allows to retrieve buckets of Stats from a streaming data set.
// the bucket indexing is based on the time.
type TimeWindowView struct {
	Time    time.Time `json:"time"`
	Count   int       `json:"count"`
	Value   float64   `json:"value"`
	EMADiff float64   `json:"ema_diff"`
	Diff    float64   `json:"diff"`
	Ratio   float64   `json:"ratio"`
	StdDev  float64   `json:"std"`
}

// NewView creates a new time view from a time bucket.
func NewView(bucket TimeBucket, index int) TimeWindowView {
	avg := bucket.Values().Stats()[index].Avg()
	ema := bucket.Values().Stats()[index].EMA()
	diff := bucket.Values().Stats()[index].Diff()
	// Note: trying to make all values relative to the price
	// so that they have a unified meaning
	return TimeWindowView{
		Time:    bucket.Time,
		Count:   bucket.Size(),
		Value:   avg,
		Diff:    diff,
		EMADiff: 100 * (ema - avg) / avg,
		Ratio:   diff / avg,
		StdDev:  100 * bucket.Values().Stats()[index].StDev() / avg,
	}
}

// TimeBucket is a wrapper for a bucket of a TimeWindow, that carries also the time index.
type TimeBucket struct {
	Bucket
	Time time.Time
}

// TimeWindow is a window indexed by the current time.
type TimeWindow struct {
	index    int64
	duration int64
	window   *Window
}

// NewTimeWindow creates a new TimeWindow with the given duration.
func NewTimeWindow(duration time.Duration) *TimeWindow {
	d := int64(duration.Seconds())
	return &TimeWindow{
		duration: d,
		window:   NewWindow(1),
	}
}

// Push adds an element to the time window.
// It will return true, if the last addition caused a bucket to close.
func (tw *TimeWindow) Push(t time.Time, v ...float64) (TimeBucket, bool) {

	// TODO : provide a inverse hash operation
	index := t.Unix() / tw.duration

	index, bucket, closed := tw.window.Push(index, v...)

	if closed {
		tw.index = index
		return TimeBucket{
			Bucket: bucket,
			Time:   time.Unix(bucket.index*tw.duration, 0),
		}, true
	}

	return TimeBucket{}, false

}

// Next returns the next timestamp for the coming window.
func (tw *TimeWindow) Next(iterations int64) time.Time {
	nextIndex := tw.index + tw.duration*(iterations+1)
	return time.Unix(nextIndex*int64(time.Second.Seconds()), 0)
}

// HistoryWindow keeps the last x buckets based on the window interval given
type HistoryWindow struct {
	window  *TimeWindow
	buckets *Ring
}

// NewHistoryWindow creates a new history window.
func NewHistoryWindow(duration time.Duration, size int) *HistoryWindow {
	return &HistoryWindow{
		window:  NewTimeWindow(duration),
		buckets: NewRing(size),
	}
}

// Push adds an element to the given time index.
// It will return true, if there was a new bucket completed at the last operation
func (h *HistoryWindow) Push(t time.Time, v ...float64) (TimeBucket, bool) {
	if bucket, ok := h.window.Push(t, v...); ok {
		h.buckets.Push(bucket)
		return bucket, true
	}
	return TimeBucket{}, false
}

// Get returns the transformed bucket value at the corresponding index.
func (h *HistoryWindow) Get(transform Transform) []interface{} {
	return h.buckets.Get(transform)
}
