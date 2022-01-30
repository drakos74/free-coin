package ml

import (
	"fmt"
	"math"
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
	Segments  map[model.Key]Segments
	Position  Position
	Debug     bool
	Benchmark bool
}

type Position struct {
	OpenValue  float64
	StopLoss   float64
	TakeProfit float64
}

// segments returns the segments that match the given parameters.
func (c Config) segments(coin model.Coin, duration time.Duration) map[model.Key]Segments {
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
	Coin      model.Coin          `json:"coin"`
	Time      time.Time           `json:"time"`
	Duration  time.Duration       `json:"duration"`
	Price     float64             `json:"price"`
	Type      model.Type          `json:"type"`
	Precision float64             `json:"precision"`
	Factor    float64             `json:"factor"`
	Weight    int                 `json:"weight"`
	Buffer    []float64           `json:"buffer"`
	Spectrum  *coin_math.Spectrum `json:"-"`
}

func (signal Signal) ToString() string {
	return fmt.Sprintf("%v : %f - %s", signal.Time, signal.Price, signal.Type.String())
}

func (signal *Signal) Filter(threshold int) bool {
	f := math.Pow(10, float64(threshold))
	return signal.Factor*f >= 1.0
}

func (signal Signal) submit(k model.Key, e *trader.ExchangeTrader, open bool, value float64) (*model.TrackedOrder, bool, trader.Action, error) {
	// find out the volume
	vol := value / signal.Price
	return e.CreateOrder(k, signal.Time, signal.Price, signal.Type, open, vol)
}

// Benchmark is responsible for tracking the performance of signals
type Benchmark struct {
	Exchange map[model.Coin]map[time.Duration]*local.Exchange
	Wallet   map[model.Coin]map[time.Duration]*trader.ExchangeTrader
}

func newBenchmarks() *Benchmark {
	return &Benchmark{
		Exchange: make(map[model.Coin]map[time.Duration]*local.Exchange),
		Wallet:   make(map[model.Coin]map[time.Duration]*trader.ExchangeTrader),
	}
}

func (b *Benchmark) assess() map[model.Key]client.Report {
	reports := make(map[model.Key]client.Report)
	for c, dd := range b.Exchange {
		for d, wallet := range dd {
			k := model.Key{
				Coin:     c,
				Duration: d,
			}
			reports[k] = wallet.Gather(true)[c]
		}
	}
	return reports
}

func (b *Benchmark) add(key model.Key, trade *model.Trade, signal Signal, config Config) (client.Report, bool, error) {
	if _, ok := b.Wallet[signal.Coin]; !ok {
		b.Wallet[signal.Coin] = make(map[time.Duration]*trader.ExchangeTrader)
		b.Exchange[signal.Coin] = make(map[time.Duration]*local.Exchange)
	}

	if _, ok := b.Wallet[signal.Coin][signal.Duration]; !ok {
		e := local.NewExchange(local.VoidLog)
		tt, err := trader.SimpleTrader(key.ToString(), json.LocalShard(), make(map[model.Coin]map[time.Duration]trader.Settings), e)
		if err != nil {
			return client.Report{}, false, fmt.Errorf("could not create trader for signal: %+v: %w", signal, err)
		}
		b.Wallet[signal.Coin][signal.Duration] = tt
		b.Exchange[signal.Coin][signal.Duration] = e
	}

	// track the log
	b.Exchange[signal.Coin][signal.Duration].Process(trade)

	_, ok, _, err := signal.submit(key, b.Wallet[signal.Coin][signal.Duration], true, config.Position.OpenValue)
	if err != nil {
		log.Err(err).Msg("could not submit signal for benchmark")
		return client.Report{}, ok, nil
	}
	if ok {
		report := b.Exchange[signal.Coin][signal.Duration].Gather(false)[signal.Coin]
		return report, ok, nil
	}
	return client.Report{}, false, nil
}
