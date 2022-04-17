package model

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/drakos74/free-coin/client"
	"github.com/drakos74/free-coin/client/local"
	coin_math "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/internal/trader"
	"github.com/rs/zerolog/log"
)

// Config defines the configuration for the collector.
type Config struct {
	Segments map[model.Key]Segments
	Position Position
	Option   Option
	Buffer   Buffer
}

func (c *Config) SetGap(coin model.Coin, gap float64) *Config {
	newSegments := make(map[model.Key]Segments)
	for k, segment := range c.Segments {
		if coin == model.AllCoins || coin == k.Coin {
			segment.Stats.Gap = gap
		}
		newSegments[k] = segment
	}
	c.Segments = newSegments
	return c
}

func (c *Config) SetPrecisionThreshold(coin model.Coin, precision float64) *Config {
	newSegments := make(map[model.Key]Segments)
	for k, segment := range c.Segments {
		if coin == model.AllCoins || coin == k.Coin {
			segment.Model.PrecisionThreshold = precision
		}
		newSegments[k] = segment
	}
	c.Segments = newSegments
	return c
}

type Option struct {
	Debug     bool
	Benchmark bool
}

type Buffer struct {
	Interval time.Duration
}

type Position struct {
	OpenValue      float64
	StopLoss       float64
	TakeProfit     float64
	TrackingConfig []*model.TrackingConfig
}

// GetSegments returns the segments that match the given parameters.
func (c *Config) GetSegments(coin model.Coin, duration time.Duration) map[model.Key]Segments {
	ss := make(map[model.Key]Segments)
	for k, s := range c.Segments {
		if k.Coin == coin && k.Duration == duration {
			ss[k] = s
		}
	}
	return ss
}

// Stats defines the statistical properties of the set
type Stats struct {
	LookBack  int     `json:"prev"`
	LookAhead int     `json:"next"`
	Gap       float64 `json:"gap"`
}

// Model defines the ml model config.
// BufferSize defines the size of the history buffer for the model
// reducing the buffer size will make the model more reactiv
// PrecisionThreshold defines the model precision in order to make use of it
// increasing the threshold will reduce trading activity, but make the trading decisions more accurate
// ModelSize defines the internal size of the model
// reducing the size will make the model more reactive
// Features defines the number of features to be used by the model
// depends strictly on the stats output
type Model struct {
	BufferSize         int     `json:"buffer"`
	PrecisionThreshold float64 `json:"precision_threshold"`
	ModelSize          int     `json:"model_size"`
	Features           int     `json:"features"`
}

func NewModel(p []float64) Model {
	return Model{
		BufferSize:         int(p[0]),
		PrecisionThreshold: p[1],
		ModelSize:          int(p[2]),
		Features:           int(p[3]),
	}
}

func EvolveModel(cc [][]float64) Model {
	mm := make([]float64, 4)
	for _, c := range cc {
		mm[0] += c[0]
		mm[1] += c[1]
		mm[2] += c[2]
		mm[3] += c[3]
	}

	for i, m := range mm {
		mm[i] = m / float64(len(cc))
	}

	return NewModel(mm)
}

const evolvePerc = 5

func (m Model) Format() string {
	return fmt.Sprintf("[%d|%.2f|%s|%s]",
		m.BufferSize,
		m.PrecisionThreshold,
		m.ModelSize,
		m.Features)
}

func (m Model) Evolve() Model {
	bf := m.BufferSize / evolvePerc
	if rand.Float64() > 0.5 {
		m.BufferSize += bf
	} else {
		m.BufferSize -= bf
	}

	pt := m.PrecisionThreshold / evolvePerc
	if rand.Float64() > 0.5 {
		m.PrecisionThreshold += pt
	} else {
		m.PrecisionThreshold -= pt
	}

	ms := m.ModelSize / evolvePerc
	if rand.Float64() > 0.5 {
		m.ModelSize += ms
	} else {
		m.ModelSize -= ms
	}

	return m
}

func (m Model) ToSlice() []float64 {
	return []float64{float64(m.BufferSize), m.PrecisionThreshold, float64(m.ModelSize), float64(m.Features)}
}

// Trader defines the trading configuration for signals
// BufferTime defines the time to wait for a signal to be confirmed by actual trading
// PriceThreshold defines the price threshold to verify the model performance within the above buffertime
// Weight defines the weight required by the model for the corresponding signal to be taken into account
type Trader struct {
	BufferTime     float64 `json:"buffer_time"`
	PriceThreshold float64 `json:"price_threshold"`
	Weight         int     `json:"weight"`
	Live           bool    `json:"live"`
}

// Segments defines the look back and ahead segment number.
// LookBack : the number of segments taken into account from the recent past
// LookAhead : the number of segments to be anticipated
// Gap : the numeric threshold for the price movement in regard to the current segment.
type Segments struct {
	Stats  Stats  `json:"stats"`
	Model  Model  `json:"model"`
	Trader Trader `json:"log"`
}

// Signal represents a signal from the ml processor.
type Signal struct {
	Key       model.Key           `json:"Key"`
	Detail    string              `json:"detail"`
	Time      time.Time           `json:"time"`
	Price     float64             `json:"price"`
	Type      model.Type          `json:"type"`
	Precision float64             `json:"precision"`
	Trend     float64             `json:"trend"`
	Factor    float64             `json:"factor"`
	Weight    int                 `json:"weight"`
	Live      bool                `json:"live"`
	Buffer    []float64           `json:"buffer"`
	Spectrum  *coin_math.Spectrum `json:"-"`
}

func (signal Signal) ToString() string {
	return fmt.Sprintf("%s | %v : %f - %s", signal.Key.ToString(), signal.Time, signal.Price, signal.Type.String())
}

func (signal Signal) String() string {
	return signal.ToString()
}

func (signal *Signal) Filter(threshold int) bool {
	f := math.Pow(10, float64(threshold))
	return signal.Factor*f >= 1.0
}

// Action defines an action of a type
type Action struct {
	Key      model.Key
	Type     model.Type
	Price    float64
	Time     time.Time
	Duration time.Duration
	PnL      float64
}

func (a Action) eval(time time.Time, price float64) Action {
	value, _ := model.PnL(a.Type, 1, a.Price, price)
	a.PnL = value
	a.Duration = time.Sub(a.Time)
	return a
}

// Benchmark is responsible for tracking the performance of signals
type Benchmark struct {
	lock     *sync.RWMutex
	Exchange map[model.Key]*local.Exchange
	Wallet   map[model.Key]*trader.ExchangeTrader
	Actions  map[model.Key][]Action
	Profit   map[model.Key][]client.Report
	Timer    map[model.Coin]int64
}

func NewBenchmarks() *Benchmark {
	return &Benchmark{
		lock:     new(sync.RWMutex),
		Exchange: make(map[model.Key]*local.Exchange),
		Wallet:   make(map[model.Key]*trader.ExchangeTrader),
		Actions:  make(map[model.Key][]Action, 0),
		Profit:   make(map[model.Key][]client.Report),
		Timer:    make(map[model.Coin]int64),
	}
}

func (b *Benchmark) Reset(coin model.Coin, key model.Key) {
	for k, _ := range b.Wallet {
		if key.Strategy == "" && k.Match(coin) {
			delete(b.Wallet, k)
			delete(b.Exchange, k)
			delete(b.Actions, k)
		} else if key == k {
			delete(b.Wallet, k)
			delete(b.Exchange, k)
			delete(b.Actions, k)
		}
	}
}

func (b *Benchmark) assess() map[model.Key][]client.Report {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.Profit
}

func (b *Benchmark) Add(key model.Key, trade model.Tick, signal Signal, config *Config) (client.Report, bool, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if _, ok := b.Wallet[key]; !ok {
		e := local.NewExchange(local.VoidLog)
		tt, err := trader.SimpleTrader(key.ToString(), json.LocalShard(), json.EventRegistry("ml-tmp-registry"), trader.Settings{
			OpenValue:  config.Position.OpenValue,
			TakeProfit: config.Position.TakeProfit,
			StopLoss:   config.Position.StopLoss,
		}, e)
		if err != nil {
			return client.Report{}, false, fmt.Errorf("could not create trader for signal: %+v: %w", signal, err)
		}
		b.Wallet[key] = tt
		b.Exchange[key] = e
		b.Actions[key] = make([]Action, 0)
	}
	if _, ok := b.Profit[key]; !ok {
		b.Profit[key] = make([]client.Report, 0)
	}

	// update the action
	action := Action{
		Key:   key,
		Type:  signal.Type,
		Price: trade.Price,
		Time:  trade.Time,
	}

	actions := b.Actions[key]
	// get last action and evaluate
	if len(actions) > 0 {
		actions[len(actions)-1] = actions[len(actions)-1].eval(action.Time, action.Price)
	}
	// Add the new action
	actions = append(actions, action)
	b.Actions[key] = actions

	// track the log
	b.Exchange[key].Process(&model.TradeSignal{
		Coin: key.Coin,
		Tick: trade,
	})

	_, ok, _, err := b.Wallet[key].CreateOrder(key, signal.Time, signal.Price, signal.Type, true, 0, trader.SignalReason, true)
	if err != nil {
		log.Err(err).Msg("could not submit signal for benchmark")
		return client.Report{}, ok, nil
	}
	if ok {
		report := b.Exchange[key].Gather(false)[key.Coin]
		// TODO : dont Reset the benchmark for now
		//sec := trade.Time.Unix()
		//g := sec / int64(4*time.Hour.Seconds())
		//if g != b.Timer[Key.Coin] {
		//	b.Timer[Key.Coin] = g
		//	report.Stamp = trade.Time
		//	b.Profit[Key] = addReport(b.Profit[Key], report, 3)
		//	b.Reset(Key.Coin)
		//}
		return report, ok, nil
	}
	return client.Report{}, false, nil
}

func addReport(ss []client.Report, s client.Report, size int) []client.Report {
	newVectors := append(ss, s)
	l := len(newVectors)
	if l > size {
		newVectors = newVectors[l-size:]
	}
	return newVectors
}