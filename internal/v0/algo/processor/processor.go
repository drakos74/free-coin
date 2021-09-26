package processor

import (
	"fmt"

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
