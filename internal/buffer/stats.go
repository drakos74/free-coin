package buffer

import (
	"fmt"
	"math"
)

// TODO: add moving average

// Stats is a set of statistical properties of a set of numbers.
type Stats struct {
	count          int // all events
	size           int // all non-zero events
	sum            float64
	first, last    float64
	min, max       float64
	mean, dSquared float64
	ema            float64
}

func MockStats(count int, sum float64, first float64, last float64) Stats {
	min := math.Min(first, last)
	max := math.Max(first, last)
	mean := (first + last) / 2
	size := count
	return Stats{
		count: count,
		size:  size,
		sum:   sum,
		first: first,
		last:  last,
		min:   min,
		max:   max,
		mean:  mean,
	}
}

// NewStats creates a new Stats.
func NewStats() *Stats {
	return &Stats{
		min: math.MaxFloat64,
	}
}

// Push adds another element to the set.
func (s *Stats) Push(v float64) {
	s.count++
	if v != 0.0 {
		s.size++
	}
	s.sum += v
	diff := (v - s.mean) / float64(s.count)
	mean := s.mean + diff
	squaredDiff := (v - mean) * (v - s.mean)
	s.dSquared += squaredDiff
	s.mean = mean

	w := 2 / float64(s.count)
	s.ema = v*w + s.ema*(1-w)

	if s.count == 1 {
		s.first = v
	}

	if s.min == 0 || s.min > v {
		s.min = v
	}

	if s.max == 0 || s.max < v {
		s.max = v
	}

	s.last = v
}

// Range returns the range of the diff.
func (s Stats) Range() (float64, float64) {
	return s.min, s.max
}

// Ratio returns the percentage of the diff.
func (s Stats) Ratio() float64 {
	if s.mean == 0 {
		return 0
	}
	return 100 * (s.last - s.first) / s.mean
}

// Avg returns the average value of the set.
func (s Stats) Avg() float64 {
	return s.mean
}

// EMA is the exponential moving average of the set.
func (s Stats) EMA() float64 {
	if math.IsNaN(s.ema) {
		return 0
	}
	return s.ema
}

// Sum returns the sum value of the set.
func (s Stats) Sum() float64 {
	return s.sum
}

// Count returns the number of elements.
func (s Stats) Count() int {
	return s.count
}

// Size returns the number of non-zero elements.
func (s Stats) Size() int {
	return s.size
}

// Diff returns the difference of max and min.
func (s Stats) Diff() float64 {
	return s.last - s.first
}

// Variance is the mathematical variance of the set.
func (s Stats) Variance() float64 {
	if s.count == 0 {
		return 0
	}
	return s.dSquared / float64(s.count)
}

// StDev is the standard deviation of the set.
func (s Stats) StDev() float64 {
	return math.Sqrt(s.Variance())
}

// SampleVariance is the sample variance of the set.
func (s Stats) SampleVariance() float64 {
	return s.dSquared / float64(s.count-1)
}

// SampleStDev is the sample standard deviation of the set.
func (s Stats) SampleStDev() float64 {
	return math.Sqrt(s.SampleVariance())
}

// StatsCollector is a collection of Stats variables.
// This enabled multi-dimensional tracking.
type StatsCollector struct {
	dim   int
	stats []*Stats
}

func (sc StatsCollector) String() string {
	return fmt.Sprintf("dim = %+v , stats = %+v",
		sc.dim, sc.stats)
}

// NewStatsCollector creates a new Stats collector.
func NewStatsCollector(dim int) *StatsCollector {
	stats := make([]*Stats, dim)
	for i := 0; i < dim; i++ {
		stats[i] = NewStats()
	}
	return &StatsCollector{
		dim:   dim,
		stats: stats,
	}
}

// Push pushes each value to the corresponding dimension.
func (sc *StatsCollector) Push(v ...float64) {
	if len(v) != sc.dim {
		panic(any(fmt.Sprintf("inconsistent dimensions of input vector %d vs config of %d", len(v), sc.dim)))
	}
	for i := 0; i < len(sc.stats); i++ {
		sc.stats[i].Push(v[i])
	}
}

func (sc StatsCollector) Stats() []*Stats {
	return sc.stats
}

// Size returns the size of the bucket.
func (sc *StatsCollector) Size() int {
	// we expect all buffer to have the same size
	return sc.stats[0].count
}

// Bucket groups together stats collectors with the same Index
type Bucket struct {
	stats *StatsCollector
	dim   int
	index int64
}

func (b Bucket) String() string {
	return fmt.Sprintf("index = %+v , stats = %+v",
		b.index, b.stats)
}

// NewBucket creates a new bucket
// with a collector of the given dimensions.
func NewBucket(id int64, dim int) Bucket {
	return Bucket{
		stats: NewStatsCollector(dim),
		dim:   dim,
		index: id,
	}
}

// Push adds an element to the bucket for the given Index.
// it returns true if the bucket has the right Index, false otherwise.
// This allows to build higher level abstractions i.e. Window etc ...
func (b *Bucket) Push(index int64, v ...float64) bool {
	if index != b.index {
		return false
	}
	b.stats.Push(v...)
	return true
}

// Size returns the number of elements in the bucket.
func (b Bucket) Size() int {
	return b.stats.Size()
}

// Values returns the bucket StatsCollector for the bucket.
func (b Bucket) Values() StatsCollector {
	return *b.stats
}

// Index returns the bucket Index.
func (b Bucket) Index() int64 {
	return b.index
}

// Flush flushes the current stats of the bucket.
func (b Bucket) Flush() ([]Stats, []Stats) {
	l := len(b.stats.Stats())
	// gather the stats
	ss := make([]Stats, l)
	// gather the stats as if it is a single event for echo purposes
	ff := make([]Stats, l)
	for i := 0; i < l; i++ {
		ss[i] = *b.stats.stats[i]
		ff[i] = Stats{
			sum:   b.stats.stats[i].mean,
			first: b.stats.stats[i].mean,
			last:  b.stats.stats[i].mean,
			min:   b.stats.stats[i].mean,
			max:   b.stats.stats[i].mean,
			mean:  b.stats.stats[i].mean,
		}
	}
	return ss, ff
}

// Window is a helper struct allowing grouping together Buckets of StatsCollectors for the given Index.
type Window struct {
	size      int64
	dim       int
	lastIndex int64
	bucket    Bucket
}

func (w Window) String() string {
	return fmt.Sprintf("size = %+v , lastIndex = %+v , bucket = %+v",
		w.size, w.lastIndex, w.bucket)
}

// NewWindow creates a new Window of the given Window size e.g. the Index range for each bucket.
func NewWindow(size int64, dim int) *Window {
	return &Window{
		size:   size,
		dim:    dim,
		bucket: NewBucket(0, dim),
	}
}

// Push adds an element to a Window at the given Index.
// returns if the Window closed, e.g. if last element initiated a new bucket.
// (Note that based on this logic we ll only know when a Window closed only on the initiation of a new one)
// (Note that the Index must be increasing for this logic to work)
// NOTE : This is not a hashmap implementation !
func (w *Window) Push(index int64, value ...float64) (int64, Bucket, bool) {

	ready := false

	lastIndex := w.lastIndex

	var last Bucket

	if index == 0 {
		// new start ...
		w.lastIndex = index
		w.bucket = NewBucket(index, w.dim)
	} else if w.size > 0 && index >= w.lastIndex+w.size {
		// start a new one
		if w.bucket.Size() > 0 {
			tmpBucket := w.bucket
			last = tmpBucket
			ready = true
		}

		w.bucket = NewBucket(index, w.dim)
		w.lastIndex = index
	}

	w.bucket.Push(w.lastIndex, value...)

	return lastIndex, last, ready

}

// Current returns the bucket Index the Window accumulates data on.
func (w *Window) Current() int64 {
	return w.lastIndex
}

// Next is the next Index at which a new bucket will be created
func (w *Window) Next() int64 {
	return w.lastIndex + w.size
}

// Bucket returns the current active bucket for the window.
func (w *Window) Bucket() Bucket {
	return w.bucket
}
