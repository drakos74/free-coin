package history

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
)

type source struct {
	registry storage.Registry
	coin     model.Coin
}

func newSource(coin model.Coin, registry storage.Registry) *source {
	return &source{
		registry: registry,
		coin:     coin,
	}
}

func (s *source) Trades(process <-chan api.Signal) (model.TradeSource, error) {
	out := make(model.TradeSource)
	trades := []model.Trade{{}}
	err := s.registry.GetAll(storage.K{
		Pair: string(s.coin),
	}, &trades)
	if err != nil {
		return nil, fmt.Errorf("could not get trades from registry: %w", err)
	}

	fmt.Printf("len(trades) = %+v\n", len(trades))
	go func() {
		defer func() {
			close(out)
		}()
		for _, t := range trades {
			out <- &t
			<-process
		}
	}()

	return out, nil
}
