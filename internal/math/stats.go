package math

import (
	"math"
)

// AggregateStats are the aggregate stats of the bucket windows.
type AggregateStats struct {
	RSI    int
	ERSI   int
	EMA    float64
	Sample int
}

// NewAggregateStats create new aggregate stats from the given indicators
func NewAggregateStats(indicators *Indicators) AggregateStats {
	rsi, ersi, count := indicators.RSI.Get()
	ema, _ := indicators.EMA.Get()
	return AggregateStats{
		RSI:    rsi,
		ERSI:   ersi,
		EMA:    ema,
		Sample: count,
	}
}

// Indicators is a stats collector based on the rsi and ema indicators
type Indicators struct {
	RSI *RSI
	EMA *EMA
}

// NewIndicators creates new stats collector
func NewIndicators() *Indicators {
	return &Indicators{
		RSI: NewRSI(),
		EMA: NewEMA(),
	}
}

// Add adds the given value to the indicator collectors.
func (s *Indicators) Add(f float64) {
	s.EMA.Add(f)
	s.RSI.Add(f)
}

// RSI is an RSI streaming calculator
type RSI struct {
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
func (rsi *RSI) Add(f float64) {
	if f > 0 {
		rsi.countPos++
	} else {
		rsi.countNeg++
	}
	rsi.values = append(rsi.values, f)
}

// Get returns the rsi for the added samples
func (rsi *RSI) Get() (value, evalue, count int) {
	// normal rsi
	var pos float64
	var neg float64
	// exponential rsi
	var epos float64
	var eneg float64
	for _, v := range rsi.values {
		if v > 0 {
			w := 2 / float64(rsi.countPos)
			epos = v*w + pos*(1-w)
			pos += v
		} else {
			w := 2 / float64(rsi.countNeg)
			eneg = v*w + neg*(1-w)
			neg += math.Abs(v)
		}
	}
	si := (pos / float64(rsi.countPos)) / (neg / float64(rsi.countNeg))

	esi := epos / math.Abs(eneg)

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
