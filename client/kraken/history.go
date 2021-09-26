package kraken

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

type History struct {
	exchange Exchange
	storage  storage.Shard
	hash     cointime.Hash
}

func NewHistory(name account.Name) (*History, error) {
	exchange := NewExchange(name)
	if client, ok := exchange.(*Exchange); ok {
		return &History{
			exchange: *client,
			storage: func(shard string) (storage.Persistence, error) {
				return json.NewJsonBlob("history", shard, false), nil
			},
			hash: cointime.NewHash(1 * time.Hour),
		}, nil
	}
	return nil, fmt.Errorf("could not build history client")
}

func (h *History) Get(from time.Time, to time.Time) (*model.TradeMap, error) {

	trades := model.NewTradeMap()

	// TODO : use the to from the arguments ...
	to = time.Now()
	i := 0
	// because of the way kraken returns the response, we need to go backwards
	for {

		timeTo := h.hash.Do(to)

		k := storage.Key{
			Hash: timeTo,
		}

		store, err := h.storage("trades")
		if err != nil {
			return nil, fmt.Errorf("could not init storage: %w", err)
		}

		var tradeMap *model.TradeMap
		err = store.Load(k, &tradeMap)
		if err != nil {
			// we ll go to the upstream
			log.Warn().
				Err(err).
				Int64("key", timeTo).
				Time("from", from).
				Time("to", to).
				Msg("could not load local trades")

			tradeMap, err = h.exchange.Api.TradesHistory(from, to)
			if err != nil {
				return nil, fmt.Errorf("could not load history trades from upstream: %w", err)
			}

			err = store.Store(k, tradeMap)
			if err != nil {
				log.Warn().Err(err).Msg("could not store history trades")
			}
		}

		trades.Append(tradeMap)

		if tradeMap.From.Before(from) || tradeMap.From.Equal(from) {
			return trades, nil
		}

		to = tradeMap.From
		i++

		if i > 10 {
			return trades, nil
		}
	}

}
