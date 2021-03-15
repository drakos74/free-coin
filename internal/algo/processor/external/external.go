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
	port          = 80
)

func Signal(user api.User, configs map[model.Coin]map[time.Duration]processor.Config) api.Processor {

	go server.NewServer("trade-view", port).
		Add(server.Live()).
		AddRoute(server.GET, server.Api, "test-get", handle(user)).
		AddRoute(server.POST, server.Api, "test-post", handle(user)).
		Run()

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
