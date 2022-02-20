package processor

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/metrics"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	Name = " void"
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
	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
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
func Process(name string, p func(trade *model.Trade) error) api.Processor {
	return ProcessWithClose(name, p, func() {})
}

// ProcessWithClose is a wrapper for a processor logic with a close execution func.
func ProcessWithClose(name string, p func(trade *model.Trade) error, shutdown func()) api.Processor {
	return func(in <-chan *model.Trade, out chan<- *model.Trade) {

		log.Info().Str("processor", name).Msg("started processor")
		defer func() {
			log.Info().Str("processor", name).Msg("closing processor")
			close(out)
			shutdown()
		}()

		for trade := range in {
			metrics.Observer.IncrementTrades(string(trade.Coin), Name, "processor")
			err := p(trade)
			if err != nil {
				log.Error().Str("processor", name).Err(err).Msg("error during processing")
			}
			out <- trade
		}
	}
}

// NoProcess is a wrapper for no processor logic
func NoProcess(name string) api.Processor {
	return Process(name, func(trade *model.Trade) error {
		return nil
	})
}
