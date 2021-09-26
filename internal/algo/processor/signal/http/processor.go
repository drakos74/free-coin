package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/drakos74/free-coin/internal/algo/processor/signal/processor"

	"github.com/drakos74/free-coin/internal/algo/processor/signal/report"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const (
	port        = 8080
	grafanaPort = 6124
)

// Propagate propagates the incoming signal to the provided channel
func Propagate(eventRegistry storage.EventRegistry, client api.Exchange, user api.User, output chan processor.Message) api.Processor {

	source := make(chan processor.Message)
	go func() {
		err := server.NewServer("trade-view", port).
			AddRoute(server.GET, server.Api, "test-get", handle(user, nil)).
			AddRoute(server.POST, server.Api, "test-post", handle(user, source)).
			Run()
		if err != nil {
			log.Error().Err(err).Msg("could not start server")
		}
	}()

	// init grafana monitor
	grafana := metrics.NewServer("grafana", grafanaPort)
	report.AddTargets(client, grafana, eventRegistry)
	// TODO : create grafana annotation for restart !
	// run the grafana server
	grafana.Run()

	// execute the logic
	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", processor.ProcessorName).Msg("closing processor")
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

func propagateMessage(id string, output chan processor.Message, message processor.Message) {
	output <- message
	log.Debug().Str("user", id).Msg("message propagated")
}

func handle(user api.User, signalChannel chan<- processor.Message) server.Handler {
	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {
		var message processor.Message
		payload, jsonErr := server.ReadJson(r, false, &message)
		if jsonErr != nil {
			// TODO : remove this irregularity
			user.Send(api.CoinClick, api.NewMessage(fmt.Sprintf("error = %v \n raw = %+v", jsonErr.Error(), payload)), nil)
			return []byte{}, http.StatusBadRequest, nil
		}
		log.Debug().Str("message", fmt.Sprintf("%+v", message)).Msg("signal received")
		if signalChannel != nil {
			signalChannel <- message
		}
		return []byte{}, http.StatusOK, nil
	}
}
