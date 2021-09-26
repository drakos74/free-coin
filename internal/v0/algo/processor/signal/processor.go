package signal

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	"github.com/rs/zerolog/log"
)

const (
	port          = 8080
	ProcessorName = "signal"
	account       = ""
)

func Propagate(user api.User, next ...Processor) api.Processor {

	source := make(chan Message)
	go func() {
		err := server.NewServer("trade-view", port).
			AddRoute(server.GET, server.Api, "test-get", handle(user, nil)).
			AddRoute(server.POST, server.Api, "test-post", handle(user, source)).
			Run()
		if err != nil {
			log.Error().Err(err).Msg("could not start server")
		}
		log.Info().Msg("started signal server")
	}()

	log.Info().Str("processor", ProcessorName).Msg("started processor")

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
				coin := model.Coin(message.Data.Ticker)
				key := message.Key()
				t, tErr := message.Type()
				if tErr != nil {
					log.Warn().
						Err(tErr).
						Str("message", fmt.Sprintf("%+v", message)).
						Str("error-field", "type").
						Msg("could not parse message")
					user.Send(account,
						api.NewMessage(processor.Error(ProcessorName, tErr)), nil)
					continue
				}
				v, _, vErr := message.Volume(minSize)
				if vErr != nil {
					log.Warn().
						Err(vErr).
						Str("message", fmt.Sprintf("%+v", message)).
						Str("error-field", "volume").
						Msg("could not parse message")
					user.Send(account,
						api.NewMessage(processor.Error(ProcessorName, vErr)), nil)
					continue
				}
				order := model.NewOrder(coin).
					Market().
					WithType(t).
					WithVolume(v).
					CreateTracked(model.Key{
						Coin:     coin,
						Duration: message.Duration(),
						Strategy: key,
					}, message.Time())
				for _, output := range next {
					go propagateMessage(user, output.User, output.Channel, order)
				}
				// TODO ... start consuming prices for the mentioned asset ...
			case trade := <-in:
				out <- trade
			}
		}
	}
}

func propagateMessage(user api.User, id string, output chan *model.TrackedOrder, order *model.TrackedOrder) {
	select {
	case output <- order:
	// all good
	default:
		log.Warn().
			Str("id", id).
			Msg("could not propagate order")
		user.Send(account,
			api.NewMessage(processor.Error(ProcessorName, fmt.Errorf("could not propagate to %s", id))), nil)
	}

	log.Debug().Str("user", id).Msg("message propagated")
}
