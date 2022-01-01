package ml

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/client"
	"github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/internal/trader"
	"github.com/rs/zerolog/log"
)

// Config defines the configuration for the collector.
type Config struct {
	Segments  map[model.Coin]map[time.Duration]Segments
	Debug     bool
	Benchmark bool
}

// Stats defines the statistical properties of the set
type Stats struct {
	LookBack  int     `json:"prev"`
	LookAhead int     `json:"next"`
	Gap       float64 `json:"gap"`
}

// Model defines the ml model config.
type Model struct {
	BufferSize         int     `json:"buffer"`
	PrecisionThreshold float64 `json:"precision_threshold"`
	Size               int     `json:"size"`
	Features           int     `json:"features"`
}

// Segments defines the look back and ahead segment number.
// LookBack : the number of segments taken into account from the recent past
// LookAhead : the number of segments to be anticipated
// Gap : the numeric threshold for the price movement in regard to the current segment.
type Segments struct {
	Stats Stats `json:"stats"`
	Model Model `json:"model"`
}

// Signal represents a signal from the ml processor.
type Signal struct {
	Coin      model.Coin    `json:"coin"`
	Time      time.Time     `json:"time"`
	Duration  time.Duration `json:"duration"`
	Price     float64       `json:"price"`
	Type      model.Type    `json:"type"`
	Precision float64       `json:"precision"`
}

func (signal Signal) submit(e *trader.ExchangeTrader) (*model.TrackedOrder, bool, error) {
	return e.CreateOrder(trader.Key{
		Coin:     signal.Coin,
		Duration: signal.Duration,
	}, signal.Time, signal.Price, signal.Type, 0.1)
}

// Benchmark is responsible for tracking the performance of strategies
type Benchmark struct {
	Exchange map[model.Coin]map[time.Duration]*local.Exchange
	Wallet   map[model.Coin]map[time.Duration]*trader.ExchangeTrader
}

func newBenchmarks() Benchmark {
	return Benchmark{
		Exchange: make(map[model.Coin]map[time.Duration]*local.Exchange),
		Wallet:   make(map[model.Coin]map[time.Duration]*trader.ExchangeTrader),
	}
}

func (b *Benchmark) assess() map[trader.Key]client.Report {
	reports := make(map[trader.Key]client.Report)
	for c, dd := range b.Exchange {
		for d, wallet := range dd {
			k := trader.Key{
				Coin:     c,
				Duration: d,
			}
			reports[k] = wallet.Gather()[c]
		}
	}
	return reports
}

func (b *Benchmark) add(trade *model.Trade, signal Signal) (client.Report, bool, error) {

	key := trader.Key{
		Coin:     signal.Coin,
		Duration: signal.Duration,
	}

	if _, ok := b.Wallet[signal.Coin]; !ok {
		b.Wallet[signal.Coin] = make(map[time.Duration]*trader.ExchangeTrader)
		b.Exchange[signal.Coin] = make(map[time.Duration]*local.Exchange)
	}

	if _, ok := b.Wallet[signal.Coin][signal.Duration]; !ok {
		e := local.NewExchange("")
		tt, err := trader.SimpleTrader(key.ToString(), json.LocalShard(), make(map[model.Coin]map[time.Duration]trader.Settings), e)
		if err != nil {
			return client.Report{}, false, fmt.Errorf("could not create trader for signal: %+v: %w", signal, err)
		}
		b.Wallet[signal.Coin][signal.Duration] = tt
		b.Exchange[signal.Coin][signal.Duration] = e
	}

	// track the trade
	b.Exchange[signal.Coin][signal.Duration].Process(trade)

	_, ok, err := signal.submit(b.Wallet[signal.Coin][signal.Duration])
	if err != nil {
		log.Err(err).Msg("could not submit signal for benchmark")
		return client.Report{}, ok, nil
	}
	if ok {
		report := b.Exchange[signal.Coin][signal.Duration].Gather()[signal.Coin]
		return report, ok, nil
	}
	return client.Report{}, false, nil
}
