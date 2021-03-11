package processor

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	Name = " void"
)

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
