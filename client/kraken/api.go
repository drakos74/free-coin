package kraken

import (
	"fmt"
	"strconv"

	krakenapi "github.com/beldur/kraken-go-api-client"
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
		if len(pair.LeverageBuy) > 0 && len(pair.LeverageSell) > 0 &&
			pair.Quote == "ZEUR" {
			if p, ok := r.converter.Coin.Alt(pair.Altname); ok {
				fmt.Printf("%v = %+v\n", p, pair)
			}
		}
	}

	r.info = symbols
	log.Trace().
		Int("pairs", len(symbols)).
		Str("exchange", "kraken").
		Msg("exchange info")

	for c, symbol := range symbols {
		log.Trace().Str("symbol", fmt.Sprintf("%+v", symbol)).Msg(fmt.Sprintf("%s", r.converter.Coin.Coin(string(c))))
	}
}

// Order opens an order in kraken.
func (r *RemoteExchange) Order(order coinmodel.Order) (*coinmodel.Order, []string, error) {

	pair, ok := r.converter.Coin.Pair(order.Coin)
	if !ok {
		return nil, nil, fmt.Errorf("could not find pair: %s", order.Coin)
	}

	// TODO : probably need another mapping here
	s, ok := r.info[coinmodel.Coin(pair.Rest)]
	if !ok {
		log.Warn().Str("pair", pair.Rest).Str("coin", string(order.Coin)).Msg("could not find exchange info")
	}

	params := make(map[string]string)

	if order.Leverage != coinmodel.NoLeverage {
		params["leverage"] = r.converter.Leverage.For(order)
	}

	if order.Price > 0 {
		// TODO : make per coin as for binance
		params["price"] = strconv.FormatFloat(order.Price, 'f', 2, 64)
	}

	direction := r.converter.Type.From(order.Type)
	oType := r.converter.OrderType.From(order.OType)

	vol := strconv.FormatFloat(order.Volume, 'f', s.LotDecimals, 64)
	response, err := r.private.AddOrder(
		pair.Rest,
		direction,
		oType,
		vol,
		params)
	if err != nil {
		return nil, nil, fmt.Errorf("could not add order '%+v' - (pair=%s,direction=%s,orderType=%s,volume=%s,decimals=%d)  : %w | %+v",
			order,
			pair, direction, oType, vol, s.LotDecimals,
			err, response)
	}

	return r.newOrder(response.Description), response.TransactionIds, nil
}
