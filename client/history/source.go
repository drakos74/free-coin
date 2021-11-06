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
	filter   func(s string) bool
}

func newSource(request Request, registry storage.Registry) *source {
	return &source{
		registry: registry,
		request:  request,
		filter:   func(s string) bool { return true },
	}
}

func (s *source) WithFilter(filter func(s string) bool) *source {
	s.filter = filter
	return s
}

func (s *source) Trades(process <-chan api.Signal) (model.TradeSource, error) {
	out := make(model.TradeSource)
	trades := []model.Trade{{}}
	err := s.registry.GetFor(storage.K{
		Pair: string(s.request.Coin),
	}, &trades, s.filter)
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
