package external

import (
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	"github.com/rs/zerolog/log"
)

const (
	ProcessorName = "external"
	port          = 6129
)

func Signal(user api.User, configs map[model.Coin]map[time.Duration]processor.Config) api.Processor {

	err := server.NewServer("trade-view", port).
		Add(server.Live()).
		AddRoute(server.GET, server.Api, "", handle(user)).
		AddRoute(server.POST, server.Api, "", handle(user)).
		Run()

	if err != nil {
		log.Error().Err(err).Msg("could not start external signal server")
	}

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for trade := range in {
			out <- trade
		}
	}

}
