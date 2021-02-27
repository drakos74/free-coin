package processor

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const Name = " void"

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
