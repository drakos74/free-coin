package processor

import (
	"fmt"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/metrics"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	Name = "void"
)

// NewStateKey creates a stats internal state key for the registry storage
func NewStateKey(processor string, key model.Key) storage.Key {
	return storage.Key{
		Pair:  processor,
		Label: key.ToString(),
	}
}

// Void is a pass-through processor that only propagates trades to the next one.
func Void(name string) api.Processor {
	processorName := fmt.Sprintf("%s-%s", name, Name)
	return func(in <-chan *model.TradeSignal, out chan<- *model.TradeSignal) {
		defer func() {
			log.Info().Str("processor", processorName).Msg("closing processor")
			close(out)
		}()
		for trade := range in {
			out <- trade
		}
	}
}

// Audit creates an audit message part for the user.
func Audit(name, msg string) string {
	return fmt.Sprintf("[%s] %s", name, msg)
}

// Error creates an error message part for the user.
func Error(name string, err error) string {
	return fmt.Sprintf("[%s] error: %s", name, err.Error())
}

// Process is a wrapper for a processor logic.
func Process(name string, p func(trade *model.TradeSignal) error) api.Processor {
	return ProcessWithClose(name, p, func() {})
}

// ProcessWithClose is a wrapper for a processor logic with a close execution func.
func ProcessWithClose(name string, p func(trade *model.TradeSignal) error, shutdown func()) api.Processor {
	return func(in <-chan *model.TradeSignal, out chan<- *model.TradeSignal) {
		log.Info().Str("processor", name).Msg("started processor")
		defer func() {
			log.Info().Str("processor", name).Msg("closing processor")
			close(out)
			shutdown()
		}()
		for trade := range in {
			metrics.Observer.IncrementTrades(string(trade.Coin), name, "source")
			err := p(trade)
			if err != nil {
				log.Error().Str("processor", name).Err(err).Msg("error during processing")
			}
			out <- trade
		}
	}
}

// ProcessBufferedWithClose is a wrapper for a processor logic with a close execution func and buffering logic.
func ProcessBufferedWithClose(name string, duration time.Duration, p func(trade *model.TradeSignal) error, shutdown func()) api.Processor {

	signalBuffer, trades := NewSignalBuffer(duration)

	go func(trades <-chan *model.TradeSignal) {
		for signal := range trades {
			coin := string(signal.Coin)
			f, _ := strconv.ParseFloat(signal.Meta.Time.Format("0102.1504"), 64)
			metrics.Observer.NoteLag(f, coin, Name, "source")
			err := p(signal)
			if err != nil {
				log.Error().Err(err).Msg("error during trade signal processing")
			}
		}
	}(trades)

	return func(in <-chan *model.TradeSignal, out chan<- *model.TradeSignal) {
		log.Info().Str("processor", name).Msg("started processor")
		defer func() {
			log.Info().Str("processor", name).Msg("closing processor")
			signalBuffer.Close()
			close(out)
			shutdown()
		}()
		for trade := range in {
			metrics.Observer.IncrementTrades(string(trade.Coin), name, "source")
			signalBuffer.Push(trade)
			out <- trade
		}
	}
}

// NoProcess is a wrapper for no processor logic
func NoProcess(name string) api.Processor {
	return Process(name, func(trade *model.TradeSignal) error {
		return nil
	})
}
