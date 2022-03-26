package history

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

type source struct {
	registry  storage.Registry
	request   Request
	filter    func(s string) bool
	threshold func(t time.Time) bool
}

func newSource(request Request, registry storage.Registry) *source {
	return &source{
		registry:  registry,
		request:   request,
		filter:    func(s string) bool { return true },
		threshold: func(t time.Time) bool { return false },
	}
}

func (s *source) WithFilter(filter func(s string) bool) *source {
	s.filter = filter
	return s
}

func (s *source) WithThreshold(threshold func(t time.Time) bool) *source {
	s.threshold = threshold
	return s
}

func (s *source) Trades(process <-chan api.Signal) (model.TradeSource, error) {
	out := make(model.TradeSource)
	trades := []model.TradeSignal{{}}
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
