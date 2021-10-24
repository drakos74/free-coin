package history

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

type source struct {
	registry storage.Registry
	request  Request
}

func newSource(request Request, registry storage.Registry) *source {
	return &source{
		registry: registry,
		request:  request,
	}
}

func (s *source) Trades(process <-chan api.Signal) (model.TradeSource, error) {
	out := make(model.TradeSource)
	trades := []model.Trade{{}}
	err := s.registry.GetAll(storage.K{
		Pair: string(s.request.Coin),
	}, &trades)
	if err != nil {
		return nil, fmt.Errorf("could not get trades from registry: %w", err)
	}

	go func() {
		defer func() {
			log.Info().Str("processor", "trades-source").Msg("closing processor")
			close(out)
		}()
		for _, t := range trades {
			if t.Time.After(s.request.From) && t.Time.Before(s.request.To) {
				out <- &t
				<-process
			}
		}
	}()

	return out, nil
}
