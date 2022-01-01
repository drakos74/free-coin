package ml

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/client"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/storage/file/json"

	"github.com/drakos74/free-coin/internal/trader"

	"github.com/sjwhitworth/golearn/base"

	"github.com/drakos74/free-coin/internal/model"
)

// Config defines the configuration for the collector.
type Config struct {
	Segments  map[model.Coin]map[time.Duration]Segments
	Debug     bool
	Benchmark bool
	Model     Model
}

// Model defines the ml model config.
type Model struct {
	BufferSize int
	Threshold  float64
	Size       int
	Features   int
}

// Segments defines the look back and ahead segment number.
// LookBack : the number of segments taken into account from the recent past
// LookAhead : the number of segments to be anticipated
// Threshold : the numeric threshold for the price movement in regard to the current segment.
type Segments struct {
	LookBack   int
	LookAhead  int
	Threshold  float64
	BufferSize int
	Precision  float64
	Model      string
	MLModel    base.Classifier
	MlDataSet  *base.DenseInstances
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

func (b *Benchmark) assess() {
	for c, dd := range b.Exchange {
		for d, wallet := range dd {
			fmt.Printf("%s | %.0f = %+v\n", c, d.Minutes(), wallet.Gather())
		}
	}
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
		fmt.Printf("%s : report = %+v\n", signal.Coin, report)
		return report, ok, nil
	}
	return client.Report{}, false, nil
}
