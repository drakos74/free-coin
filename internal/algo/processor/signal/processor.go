package signal

import (
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

func Propagate(eventRegistry storage.EventRegistry, client api.Exchange, user api.User, output chan Message) api.Processor {

	source := make(chan Message)
	go server.NewServer("trade-view", port).
		AddRoute(server.GET, server.Api, "test-get", handle(user, nil)).
		AddRoute(server.POST, server.Api, "test-post", handle(user, source)).
		Run()

	// init grafana monitor
	grafana := metrics.NewServer("grafana", grafanaPort)
	addTargets(client, grafana, eventRegistry)
	// run the grafana server
	grafana.Run()

	// execute the logic
	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for {
			select {
			case message := <-source:
				// propagate message to others ...
				output <- message
				// TODO ... start consuming prices for the mentioned asset ...
			case trade := <-in:
				out <- trade
			}
		}
	}
}
