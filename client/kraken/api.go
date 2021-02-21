package kraken

import (
	"fmt"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/client/kraken/model"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

// RemoteClient defines a remote api for interaction with kraken exchange.
type RemoteClient struct {
	converter model.Converter
	Interval  time.Duration
	public    *krakenapi.KrakenAPI
}

// AssetPairs retrieves the active asset pairs with their trading details from kraken.
func (r *RemoteClient) AssetPairs() (*krakenapi.AssetPairsResponse, error) {
	return r.public.AssetPairs()
}

// Trades retrieves the next trades batch from kraken.
func (r *RemoteClient) Trades(coin coinmodel.Coin, since int64) (*model.TradeBatch, error) {
	pair := r.converter.Coin.Pair(coin)
	log.Trace().
		Str("method", "Count").
		Str("pair", pair).
		Int64("Since", since).
		Msg("calling remote")
	// TODO : avoid the duplicate iteration on the trades
	response, err := r.public.Trades(pair, since)
	if err != nil {
		return nil, fmt.Errorf("could not get trades from kraken: %w", err)
	}
	return r.transform(pair, r.Interval, response)
}

func (r *RemoteClient) transform(pair string, interval time.Duration, response *krakenapi.TradesResponse) (*model.TradeBatch, error) {
	l := len(response.Trades)
	if l == 0 {
		return &model.TradeBatch{
			Trades: []coinmodel.Trade{},
			Index:  response.Last,
		}, nil
	}
	last := cointime.FromNano(response.Last)
	var live bool
	if time.Since(last) < interval {
		live = true
	}
	trades := make([]coinmodel.Trade, l)
	for i := 0; i < l; i++ {
		trades[i] = r.newTrade(pair, i == l-1, live, response.Trades[i])
	}
	return &model.TradeBatch{
		Trades: trades,
		Index:  response.Last,
	}, nil
}

// RemoteExchange implements the exchange api for kraken.
type RemoteExchange struct {
	converter model.Converter
	private   *krakenapi.KrakenAPI
}

// Order opens an order in kraken.
func (r *RemoteExchange) Order(order coinmodel.Order) (*coinmodel.Order, []string, error) {
	params := make(map[string]string)

	if order.Leverage != coinmodel.NoLeverage {
		params["leverage"] = r.converter.Leverage.For(order)
	}

	if order.Price > 0 {
		params["price"] = coinmodel.Price.Format(order.Coin, order.Price)
	}

	response, err := r.private.AddOrder(
		r.converter.Coin.Pair(order.Coin),
		r.converter.Type.From(order.Type),
		r.converter.OrderType.From(order.OType),
		coinmodel.Volume.Format(order.Coin, order.Volume),
		params)
	if err != nil {
		return nil, nil, fmt.Errorf("could not add order '%+v': %w", order, err)
	}

	return r.newOrder(response.Description), response.TransactionIds, nil
}

// Close closes the kraken client.
func (r *RemoteClient) Close() error {
	return nil
}
