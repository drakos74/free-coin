package signal

import (
	"fmt"

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
	// TODO : create grafana annotation for restart !
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
				log.Debug().
					Str("message", fmt.Sprintf("%+v", message)).
					Msg("received message")
				// propagate message to others ...
				go propagateMessage("", output, message)
				// TODO ... start consuming prices for the mentioned asset ...
			case trade := <-in:
				out <- trade
			}
		}
	}
}

func propagateMessage(id string, output chan Message, message Message) {
	output <- message
	log.Debug().Str("user", id).Msg("message propagated")
}
