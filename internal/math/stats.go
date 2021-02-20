package math

import (
	"math"
)

// AggregateStats are the aggregate stats of the bucket windows.
type AggregateStats struct {
	RSI    int
	ERSI   int
	EMA    float64
	RSIMA  float64
	Sample int
}

// NewAggregateStats create new aggregate stats from the given indicators
func NewAggregateStats(indicators *Indicators) AggregateStats {
	rsi, ersi, count := indicators.RSI.Get()
	ema, _ := indicators.EMA.Get()
	rsima, _ := indicators.RSIMA.Get()
	return AggregateStats{
		RSI:    rsi,
		ERSI:   ersi,
		EMA:    ema,
		RSIMA:  rsima,
		Sample: count,
	}
}

// Indicators is a stats collector based on the rsi and ema indicators
type Indicators struct {
	RSI   *RSI
	EMA   *EMA
	RSIMA *EMA
}

// NewIndicators creates new stats collector
func NewIndicators() *Indicators {
	return &Indicators{
		RSI:   NewRSI(),
		EMA:   NewEMA(),
		RSIMA: NewEMA(),
	}
}

// Add adds the given value to the indicator collectors.
func (s *Indicators) Add(f float64) {
	s.EMA.Add(f)
	v, _, _ := s.RSI.Add(f)
	s.RSIMA.Add(float64(v))
}

// RSI is an RSI streaming calculator
type RSI struct {
	pos      float64
	neg      float64
	countPos int
	countNeg int
	values   []float64
}

// NewRSI creates a new RSI calculator
func NewRSI() *RSI {
	return &RSI{
		values: make([]float64, 0),
	}
}

// Add adds another value for the RSI calculation and returns the intermediate result.
func (rsi *RSI) Add(f float64) (value, evalue, count int) {
	if f > 0 {
		rsi.pos += f
		rsi.countPos++
	} else {
		rsi.neg += math.Abs(f)
		rsi.countNeg++
	}
	rsi.values = append(rsi.values, f)
	return rsi.Get()
}

// Get returns the rsi for the added samples
func (rsi *RSI) Get() (value, evalue, count int) {

	si := (rsi.pos / float64(rsi.countPos)) / (rsi.neg / float64(rsi.countNeg))

	// exponential rsi
	var pos float64
	var neg float64
	for _, v := range rsi.values {
		if v > 0 {
			w := 2 / float64(rsi.countPos)
			pos = v*w + pos*(1-w)
		} else {
			w := 2 / float64(rsi.countNeg)
			neg = v*w + neg*(1-w)
		}
	}

	esi := pos / neg

	return int(math.Round(100 - (100 / (1 + si)))), int(math.Round(100 - (100 / (1 + esi)))), rsi.countPos + rsi.countNeg
}

// EMA calculates the exponential moving average for the given sample.
type EMA struct {
	values []float64
}

// NewEMA creates a new EMA
func NewEMA() *EMA {
	return &EMA{values: make([]float64, 0)}
}

// Add adds a value to the sample
func (ema *EMA) Add(f float64) {
	ema.values = append(ema.values, f)
}

// Get returns the calculated ema
func (ema *EMA) Get() (value float64, count int) {
	l := float64(len(ema.values))
	var result float64
	for _, v := range ema.values {
		w := 2 / l
		result = v*w + result*(1-w)
	}
	return result, len(ema.values)
}
