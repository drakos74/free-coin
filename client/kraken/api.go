package kraken

import (
	"fmt"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/client/kraken/model"
	"github.com/drakos74/free-coin/client/kraken/public"
	"github.com/drakos74/free-coin/internal/api"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

// Remote defines a remote api for interaction with kraken exchange.
type Remote struct {
	Interval   time.Duration
	PublicApi  *krakenapi.KrakenAPI
	PrivateApi *krakenapi.KrakenAPI
}

func (r *Remote) Trades(coin api.Coin, since int64) (*model.TradeBatch, error) {
	pair := model.Pair(coin)
	log.Trace().
		Str("method", "Count").
		Str("pair", pair).
		Int64("Since", since).
		Msg("calling remote")
	// TODO : avoid the duplicate iteration on the trades
	response, err := r.PublicApi.Trades(pair, since)
	if err != nil {
		return nil, fmt.Errorf("could not get trades from kraken: %w", err)
	}
	return transform(pair, r.Interval, response)
}

func transform(pair string, interval time.Duration, response *krakenapi.TradesResponse) (*model.TradeBatch, error) {
	l := len(response.Trades)
	if l == 0 {
		return &model.TradeBatch{
			Trades: []api.Trade{},
			Index:  response.Last,
		}, nil
	}
	last := cointime.FromNano(response.Last)
	var live bool
	if time.Since(last) < interval {
		live = true
	}
	trades := make([]api.Trade, l)
	for i := 0; i < l; i++ {
		trades[i] = public.NewTrade(pair, i == l-1, live, response.Trades[i])
	}
	return &model.TradeBatch{
		Trades: trades,
		Index:  response.Last,
	}, nil
}

func (r *Remote) AssetPairs() (*krakenapi.AssetPairsResponse, error) {
	return r.PublicApi.AssetPairs()
}

func (r *Remote) Close() error {
	return nil
}
