package model

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/drakos74/free-coin/client"
	"github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/emoji"
	coin_math "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/internal/trader"
	"github.com/rs/zerolog/log"
)

type SegmentConfig map[model.Key]Segments

type ConfigSegment func(coin model.Coin) func(cfg SegmentConfig) SegmentConfig

func (sgm SegmentConfig) AddConfig(add ...func(cfg SegmentConfig) SegmentConfig) SegmentConfig {
	cfg := sgm
	for _, fn := range add {
		cfg = fn(cfg)
	}
	return cfg
}

// Config defines the configuration for the collector.
type Config struct {
	// Segments defines the configuration for different segments of analysis
	// it can relate to same coin with different observation intervals or different coins
	Segments SegmentConfig
	// Position defines the configuration for the position tracking and closing decisions
	Position Position
	// Option defines the algorithm running config options like logging level etc ...
	Option Option
	// Buffer defines the minimal grouping interval for observing positions
	Buffer Buffer
	// Segment defines the minimal grouping interval for market analysis
	Segment Buffer
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

func (c *Config) SetPrecisionThreshold(coin model.Coin, network string, precision float64) *Config {
	newSegments := make(map[model.Key]Segments)
	for k, segment := range c.Segments {
		if coin == model.AllCoins || coin == k.Coin {
			models := make([]Model, len(segment.Stats.Model))
			for i, m := range segment.Stats.Model {
				if m.Detail.Type == network || network == "all" {
					m.Threshold = precision
					models[i] = m
				}
			}
			segment.Stats.Model = models
		}
		newSegments[k] = segment
	}
	c.Segments = newSegments
	return c
}

// Model defines the ml model config.
// BufferSize defines the size of the history buffer for the model
// reducing the buffer size will make the model more reactiv
// Threshold defines the model precision in order to make use of it
// increasing the threshold will reduce trading activity, but make the trading decisions more accurate
// Spread is used to quantize outputs of the model to make the output more simple
// Size defines the internal size of the model
// reducing the size will make the model more reactive
// Features defines the number of features to be used by the model
// depends strictly on the stats output
// MaxEpochs defines the maximmum epochs for the training process
// LearningRate defines the learning rate for the model
type Model struct {
	Detail       Detail  `json:"type"`
	BufferSize   int     `json:"buffer"`
	Threshold    float64 `json:"threshold"`
	Spread       float64 `json:"spread"`
	Size         []int   `json:"size"`
	Features     []int   `json:"features"`
	MaxEpochs    int     `json:"max_epochs"`
	LearningRate float64 `json:"learning_rate"`
	Multi        bool    `json:"multi"`
}

// NewConfig defines a numeric way to initialise the config
// we will use it mostly to do hyperparameter tuning
func NewConfig(p ...[]float64) Model {
	return Model{
		BufferSize:   int(p[0][0]),
		Threshold:    p[1][0],
		Size:         coin_math.ToInt(p[2]),
		Features:     coin_math.ToInt(p[3]),
		MaxEpochs:    int(p[4][0]),
		LearningRate: p[5][0],
	}
}

type Performance struct {
	Config [][]float64
	Score  float64
}

type ByScore []Performance

func (p ByScore) Len() int           { return len(p) }
func (p ByScore) Less(i, j int) bool { return p[i].Score < p[j].Score }
func (p ByScore) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

//func EvolveModel(cc []Performance, evolution []bool) Config {
//	mm := make([][]float64, 6)
//	for _, c := range cc {
//		mm[0] = EvolveInt(c.Config[0][0])
//		mm[1] += c.Config[1]
//		mm[2] += c.Config[2]
//		mm[3] += c.Config[3]
//	}
//
//	for i, m := range mm {
//		mm[i] = m / float64(len(cc))
//	}
//
//	return NewConfig(mm)
//}
//
//func MergeModels(cc []Performance) Config {
//	mm := make([]float64, 4)
//	for _, c := range cc {
//		mm[0] += c.Config[0]
//		mm[1] += c.Config[1]
//		mm[2] += c.Config[2]
//		mm[3] += c.Config[3]
//	}
//
//	for i, m := range mm {
//		mm[i] = m / float64(len(cc))
//	}
//
//	return NewMLConfig(mm)
//}

const evolvePerc = 0.05

func EvolveAsInt(i int, r float64) int {
	if r == 0.0 {
		r = rand.Float64()
	}
	ii := float64(i) * evolvePerc
	if r > 0.5 {
		i += int(ii)
	} else {
		i -= int(ii)
	}
	return i
}

// EvolveFloat evolves a float number
// f is the number to be used as base
// r is the bias for producing a bigger or smaller number than the initial
// limit is the highest possible value for the new generated float. It acts as a cap
func EvolveFloat(f float64, r float64, limit float64) float64 {
	ff := f * evolvePerc
	if r == 0.0 {
		r = rand.Float64()
	}
	if r > 0.5 {
		f += ff
	} else {
		f -= ff
	}
	res := math.Round(1000*f) / 1000
	if res > limit {
		return limit
	}
	return res
}

func (m Model) Format() string {
	return fmt.Sprintf("(%s:%s)[buffer:%d|threshold:%.2f|spread:%.2f.f|size:%d|features:%d]",
		m.Detail.Type, m.Detail.Hash,
		m.BufferSize,
		m.Threshold,
		m.Spread,
		m.Size,
		m.Features)
}

func (m Model) ToSlice() [][]float64 {
	return [][]float64{
		{float64(m.BufferSize)},
		{m.Threshold},
		coin_math.ToFloat(m.Size),
		coin_math.ToFloat(m.Features),
		{float64(m.MaxEpochs)},
		{m.LearningRate}}
}

type Option struct {
	Trace     map[string]bool
	Log       bool
	Debug     bool
	Benchmark bool
}

type Buffer struct {
	Interval time.Duration
	History  bool
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
	Live      bool    `json:"live"`
	Model     []Model `json:"model"`
}

func (s Stats) Format() string {
	return fmt.Sprintf("[%d:%d] %.2f", s.LookBack, s.LookAhead, s.Gap)
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

func (t Trader) Format() string {
	return fmt.Sprintf("%s", emoji.MapOpen(t.Live))
}

// Segments defines the look back and ahead segment number.
// LookBack : the number of segments taken into account from the recent past
// LookAhead : the number of segments to be anticipated
// Gap : the numeric threshold for the price movement in regard to the current segment.
type Segments struct {
	Stats  Stats  `json:"stats"`
	Trader Trader `json:"log"`
}

// Signal represents a signal from the ml processor.
type Signal struct {
	Key       model.Key           `json:"Index"`
	Detail    Detail              `json:"detail"`
	Time      time.Time           `json:"time"`
	Price     float64             `json:"price"`
	Type      model.Type          `json:"type"`
	Precision float64             `json:"precision"`
	Gap       float64             `json:"gap"`
	Trend     float64             `json:"trend"`
	Factor    float64             `json:"factor"`
	Weight    int                 `json:"weight"`
	Live      bool                `json:"live"`
	Buffer    []float64           `json:"buffer"`
	Spectrum  *coin_math.Spectrum `json:"-"`
}

// Detail defines the network details to distinguish between different objects
type Detail struct {
	Type  string `json:"type"`
	Hash  string `json:"hash"`
	Index int    `json:"index"`
}

func (detail Detail) ToString() string {
	return fmt.Sprintf("%s_%s", detail.Type, detail.Hash)
}

func NetworkType(n string) Detail {
	return Detail{
		Type: n,
	}
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
	pnl, _, _ := model.PnL(a.Type, 1, a.Price, price)
	a.PnL = pnl
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
		}, e, nil)
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

	_, ok, _, err := b.Wallet[key].CreateOrder(key, signal.Time, signal.Price, signal.Type, true, 0, trader.SignalReason, true, nil)
	if err != nil {
		log.Err(err).Msg("could not submit signal for benchmark")
		return client.Report{}, ok, nil
	}
	if ok {
		report := b.Exchange[key].Gather(false)[key.Coin]
		// TODO : dont Reset the benchmark for now
		//sec := trade.Time.Unix()
		//g := sec / int64(4*time.Hour.Seconds())
		//if g != b.Timer[Index.Coin] {
		//	b.Timer[Index.Coin] = g
		//	report.Stamp = trade.Time
		//	b.Profit[Index] = addReport(b.Profit[Index], report, 3)
		//	b.Reset(Index.Coin)
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
