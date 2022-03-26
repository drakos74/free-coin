package buffer

import (
	"fmt"
	"sync"
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

func (tw TimeWindow) String() string {
	return fmt.Sprintf("index = %+v , duration = %+v , window = %+v", tw.Index, tw.Duration, tw.window)
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
func (tw *TimeWindow) Push(t time.Time, v ...float64) (TimeBucket, bool) {

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

func (h HistoryWindow) String() string {
	return fmt.Sprintf("window = %+v\nbuckets = %+v", h.Window, h.buckets)
}

// NewHistoryWindow creates a new history Window.
func NewHistoryWindow(duration time.Duration, size int) HistoryWindow {
	return HistoryWindow{
		Window:  NewTimeWindow(duration),
		buckets: NewRing(size),
	}
}

// TODO : make this a channel in order to time the bucket correctly

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

// Extract extracts the bucket values as a series
func (h HistoryWindow) Extract(index int, extract func(b TimeWindowView) float64) ([]float64, []float64, error) {
	xx := make([]float64, 0)
	yy := make([]float64, 0)
	t0 := 0.0
	buckets := h.buckets.Get(func(bucket interface{}) interface{} {
		if b, ok := bucket.(TimeBucket); ok {
			return NewView(b, index)
		}
		return bucket
	})

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
			return nil, nil, fmt.Errorf("window does not contain timebuckets")
		}
	}
	return xx, yy, nil
}

// Polynomial evaluates the polynomial regression
// for the given polynomial degree
// and based on the value extracted from the TimeBucket
// scaled at the corresponding time duration.
func (h HistoryWindow) Polynomial(index int, extract func(b TimeWindowView) float64, degree int) ([]float64, error) {
	xx, yy, err := h.Extract(index, extract)

	if err != nil {
		return nil, fmt.Errorf("could not extarct series: %w", err)
	}
	if len(yy) < degree+1 {
		return nil, fmt.Errorf("not enough buckets (%d out of %d) to apply polynomial regression for %d",
			len(yy),
			degree+1,
			degree)
	}
	return math.Fit(xx, yy, degree)
}

// Values returns the values of the time window in a slice
func (h HistoryWindow) Values(index int, extract func(b TimeWindowView) float64) ([]float64, error) {
	_, yy, err := h.Extract(index, extract)
	if err != nil {
		return nil, fmt.Errorf("could not extarct series: %w", err)
	}
	return yy, nil
}

// Rat returns the avg value
func Rat(b TimeWindowView) float64 {
	return b.Ratio
}

// Avg returns the avg value
func Avg(b TimeWindowView) float64 {
	return b.Value
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

// StatsMessage defines a stats instance
type StatsMessage struct {
	OK       bool          `json:"ok"`
	Time     time.Time     `json:"Time"`
	Duration time.Duration `json:"Duration"`
	Dim      int64         `json:"Dimensions"`
	Stats    []Stats       `json:"-"`
}

type IntervalWindow struct {
	Time     time.Time         `json:"Time"`
	Duration time.Duration     `json:"Duration"`
	Dim      int64             `json:"Dimensions"`
	window   *Window           `json:"-"`
	stats    chan StatsMessage `json:"-"`
	lock     *sync.Mutex       `json:"-"`
}

// NewIntervalWindow creates a new IntervalWindow with the given Duration.
func NewIntervalWindow(dim int64, duration time.Duration) (*IntervalWindow, <-chan StatsMessage) {
	stats := make(chan StatsMessage)

	iw := &IntervalWindow{
		Duration: duration,
		Dim:      dim,
		stats:    stats,
		lock:     new(sync.Mutex),
	}

	iw.exec()

	return iw, stats
}

// Push adds an element to the interval Window.
func (iw *IntervalWindow) Push(t time.Time, v ...float64) {
	iw.lock.Lock()
	defer iw.lock.Unlock()
	if iw.window == nil {
		iw.window = NewWindow(iw.Dim)
		iw.Time = t
	}
	iw.window.Push(1, v...)
}

// Flush flushes the current bucket contents
func (iw *IntervalWindow) Flush() {
	iw.lock.Lock()
	defer iw.lock.Unlock()

	// if we did not have any event
	if iw.window == nil {
		iw.stats <- StatsMessage{}
		return
	}

	stats := iw.window.bucket.Flush()
	iw.window = nil
	iw.stats <- StatsMessage{
		OK:       true,
		Time:     iw.Time,
		Duration: iw.Duration,
		Dim:      iw.Dim,
		Stats:    stats,
	}
}

// exec initiates the execution
func (iw *IntervalWindow) exec() {
	time.AfterFunc(iw.Duration, func() {
		iw.Flush()
		iw.exec()
	})
}

// Close closes the channel
func (iw *IntervalWindow) Close() error {
	close(iw.stats)
	return nil
}

type BatchWindow struct {
	Window  *IntervalWindow `json:"Window"`
	buckets *Ring           `json:"-"`
}

// NewBatchWindow creates a new batch Window.
func NewBatchWindow(dim int64, duration time.Duration, size int) (BatchWindow, <-chan []StatsMessage) {
	stats := make(chan []StatsMessage)
	iw, stat := NewIntervalWindow(dim, duration)
	bw := BatchWindow{
		Window:  iw,
		buckets: NewRing(size),
	}

	ss := make([]StatsMessage, 0)
	go func() {
		for s := range stat {
			if len(ss) >= size {
				ss = ss[1:]
			}
			ss = append(ss, s)
			stats <- ss
		}
		close(stats)
	}()

	return bw, stats
}

// TODO : make this a channel in order to time the bucket correctly

// Push adds an element to the given time Index.
// It will return true, if there was a new bucket completed at the last operation
func (bw BatchWindow) Push(t time.Time, v ...float64) {
	bw.Window.Push(t, v...)
}

// Close closes the channel
func (bw BatchWindow) Close() error {
	return bw.Window.Close()
}
