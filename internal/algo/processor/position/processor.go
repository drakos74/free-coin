package position

import (
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	Name = "position-tracker"
)

// Processor is the position processor main routine.
func Processor(index api.Index) func(u api.User, e api.Exchange) api.Processor {
	return func(u api.User, e api.Exchange) api.Processor {
		t := newTracker(index, u, e)
		go t.track()

		return func(in <-chan *model.Trade, out chan<- *model.Trade) {
			log.Info().Str("processor", Name).Msg("started processor")
			for trade := range in {
				//fmt.Printf("trade = %+v\n", trade)
				// TODO : track trade density
				out <- trade
			}
		}
	}
}
