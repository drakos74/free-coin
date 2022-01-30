package kraken

import (
	"fmt"
	"strconv"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/client/kraken/model"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// RemoteExchange implements the exchange api for kraken.
type RemoteExchange struct {
	*baseSource
	private *krakenapi.KrakenAPI
	info    map[coinmodel.Coin]krakenapi.AssetPairInfo
}

func (r *RemoteExchange) getInfo() {
	pairs, err := r.private.AssetPairs()
	if err != nil {
		log.Error().Str("exchange", "kraken").Msg("could not get exchange info")
	}

	symbols := make(map[coinmodel.Coin]krakenapi.AssetPairInfo)

	for c, pair := range *pairs {
		coin := coinmodel.Coin(c)
		symbols[coin] = pair
	}

	r.info = symbols
	log.Info().
		Int("pairs", len(symbols)).
		Str("exchange", "kraken").
		Msg("exchange info")

	for c, symbol := range symbols {
		log.Trace().Str("symbol", fmt.Sprintf("%+v", symbol)).Msg(fmt.Sprintf("%s", r.converter.Coin.Coin(string(c))))
	}
}

func (r *RemoteExchange) TradesHistory(from time.Time, to time.Time) (*coinmodel.TradeMap, error) {
	response, err := r.private.TradesHistory(from.Unix(), to.Unix(), map[string]string{
		"trades": "true",
	})

	if err != nil {
		return nil, fmt.Errorf("could not get trade history from kraken: %w", err)
	}

	if response.Count == 0 {
		return coinmodel.NewTradeMap(), nil
	}

	trades := make([]coinmodel.Trade, 0)
	for k, trade := range response.Trades {
		trades = append(trades, model.NewHistoryTrade(k, trade))
	}
	return coinmodel.NewTradeMap(trades...), nil
}

// Order opens an order in kraken.
func (r *RemoteExchange) Order(order coinmodel.Order) (*coinmodel.Order, []string, error) {

	pair := r.converter.Coin.Pair(order.Coin)

	s, ok := r.info[coinmodel.Coin(pair)]
	if !ok {
		log.Warn().Str("pair", pair).Str("coin", string(order.Coin)).Msg("could not find exchange info")
	}

	params := make(map[string]string)

	if order.Leverage != coinmodel.NoLeverage {
		params["leverage"] = r.converter.Leverage.For(order)
	}

	if order.Price > 0 {
		// TODO : make poer coin as for binance
		params["price"] = strconv.FormatFloat(order.Price, 'f', 2, 64)
		//coinmodel.Price.Format(order.Coin, order.Price)
	}

	response, err := r.private.AddOrder(
		r.converter.Coin.Pair(order.Coin),
		r.converter.Type.From(order.Type),
		r.converter.OrderType.From(order.OType),
		strconv.FormatFloat(order.Volume, 'f', s.PairDecimals, 64),
		params)
	if err != nil {
		return nil, nil, fmt.Errorf("could not add order '%+v': %w", order, err)
	}

	return r.newOrder(response.Description), response.TransactionIds, nil
}
