package ml

import (
	"fmt"
	"math"
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
	Test      bool
}

type Position struct {
	OpenValue      float64
	StopLoss       float64
	TakeProfit     float64
	TrackingConfig []*model.TrackingConfig
}

// segments returns the segments that match the given parameters.
func (c *Config) segments(coin model.Coin, duration time.Duration) map[model.Key]Segments {
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

// Trader defines the trading configuration for signals
// BufferTime defines the time to wait for a signal to be confirmed by actual trading
// PriceThreshold defines the price threshold to verify the model performance within the above buffertime
// Weight defines the weight required by the model for the corresponding signal to be taken into account
type Trader struct {
	BufferTime     float64 `json:"buffer_time"`
	PriceThreshold float64 `json:"price_threshold"`
	Weight         int     `json:"weight"`
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
	Key       model.Key           `json:"key"`
	Time      time.Time           `json:"time"`
	Price     float64             `json:"price"`
	Type      model.Type          `json:"type"`
	Precision float64             `json:"precision"`
	Factor    float64             `json:"factor"`
	Weight    int                 `json:"weight"`
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

func newBenchmarks() *Benchmark {
	return &Benchmark{
		lock:     new(sync.RWMutex),
		Exchange: make(map[model.Key]*local.Exchange),
		Wallet:   make(map[model.Key]*trader.ExchangeTrader),
		Actions:  make(map[model.Key][]Action, 0),
		Profit:   make(map[model.Key][]client.Report),
		Timer:    make(map[model.Coin]int64),
	}
}

func (b *Benchmark) reset(coin model.Coin) {
	for k, _ := range b.Wallet {
		if k.Match(coin) {
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

func (b *Benchmark) add(key model.Key, trade model.Tick, signal Signal, config *Config) (client.Report, bool, error) {
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
	// add the new action
	actions = append(actions, action)
	b.Actions[key] = actions

	// track the log
	b.Exchange[key].Process(&model.TradeSignal{
		Coin: key.Coin,
		Tick: trade,
	})

	_, ok, _, err := b.Wallet[key].CreateOrder(key, signal.Time, signal.Price, signal.Type, true, 0, trader.SignalReason)
	if err != nil {
		log.Err(err).Msg("could not submit signal for benchmark")
		return client.Report{}, ok, nil
	}
	if ok {
		report := b.Exchange[key].Gather(false)[key.Coin]
		sec := trade.Time.Unix()
		g := sec / int64(4*time.Hour.Seconds())
		if g != b.Timer[key.Coin] {
			b.Timer[key.Coin] = g
			report.Stamp = trade.Time
			b.Profit[key] = addReport(b.Profit[key], report, 3)
			b.reset(key.Coin)
		}
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
