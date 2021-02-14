package kraken

import (
	"fmt"

	"github.com/drakos74/free-coin/coinapi"
	"github.com/drakos74/free-coin/kraken/api"
	"github.com/drakos74/free-coin/kraken/config"
	"github.com/drakos74/free-coin/kraken/private"
	"github.com/drakos74/free-coin/kraken/public"

	"github.com/drakos74/free-coin/internal/algo/model"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

type Remote struct {
	publicApi  *krakenapi.KrakenAPI
	privateApi *krakenapi.KrakenAPI
}

func Api() *Remote {
	publicAPI := krakenapi.New("KEY", "SECRET")
	privateAPI := krakenapi.New(config.Key, config.Secret)
	return &Remote{
		publicApi:  publicAPI,
		privateApi: privateAPI,
	}

}

func (r *Remote) Trades(coin model.Coin, since int64) (*model.TradeBatch, error) {
	pair := api.Pair(coin)
	log.Trace().
		Str("method", "Count").
		Str("pair", pair).
		Int64("Since", since).
		Msg("calling remote")
	// TODO : avoid the duplicate iteration on the trades
	response, err := r.publicApi.Trades(pair, since)
	if err != nil {
		return nil, fmt.Errorf("could not get trades from kraken: %w", err)
	}
	return transform(pair, response)
}

func transform(pair string, response *krakenapi.TradesResponse) (*model.TradeBatch, error) {
	l := len(response.Trades)
	if l == 0 {
		return &model.TradeBatch{
			Trades: []model.Trade{},
			Index:  response.Last,
		}, nil
	}
	trades := make([]model.Trade, l)
	for i := 0; i < l; i++ {
		trades[i] = public.NewTrade(pair, i == l-1, response.Trades[i])
	}
	return &model.TradeBatch{
		Trades: trades,
		Index:  response.Last,
	}, nil
}

func (r *Remote) OpenOrders() (*model.OrderBatch, error) {
	response, err := r.privateApi.OpenOrders(map[string]string{"trades": "true"})

	if err != nil {
		return nil, fmt.Errorf("could not get open orders from kraken: %w", err)
	}

	if response.Count == 0 {
		return &model.OrderBatch{
			Orders: []model.Order{},
			Index:  time.Now().UnixSecond,
		}, nil
	}

	orders := make([]model.Order, response.Count)

	i := 0
	for k, order := range response.Open {
		orders[i] = private.NewOrder(k, order)
		i++
	}
	return &coinapi.OrderBatch{
		Orders: orders,
		Index:  time.Now().UnixSecond,
	}, nil
}

func (r *Remote) TradesHistory() (*coinapi.TradeBatch, error) {
	response, err := r.privateApi.TradesHistory(time.ThisWeek(), time.Now().UnixSecond, map[string]string{
		"trades": "true",
	})

	if err != nil {
		return nil, fmt.Errorf("could not get trade history from kraken: %w", err)
	}

	if response.Count == 0 {
		return &coinapi.TradeBatch{
			Trades: []coinapi.Trade{},
			Index:  time.Now().UnixSecond,
		}, nil
	}

	trades := make([]coinapi.Trade, response.Count)
	i := 0
	for k, trade := range response.Trades {
		trades[i] = private.NewHistoryTrade(k, trade)
		i++
	}
	return &coinapi.TradeBatch{
		Trades: trades,
		Index:  time.Now().UnixSecond,
	}, nil
}

func (r *Remote) Balance() (*coinapi.AccountBalance, error) {
	resp := &coinapi.AccountBalance{}
	err := r.privateApi.Balance(resp)
	return resp, err
}

func (r *Remote) AssetPairs() (*krakenapi.AssetPairsResponse, error) {
	return r.publicApi.AssetPairs()
}

func (r *Remote) order(pair string, dir coinapi.Type, volume float64) (*krakenapi.AddOrderResponse, error) {
	return r.privateApi.AddOrder(pair, dir.String(), "market", processor.Round(volume), make(map[string]string))
}

func (r *Remote) Order(order coinapi.Order) (*coinapi.Order, []string, error) {
	params := make(map[string]string)

	if order.Leverage != coinapi.NoLeverage {
		// TODO : check the max leverage (?)
		params["leverage"] = api.LeverageString(order.Leverage)
	}

	if order.Price > 0 {
		params["price"] = order.Coin.FormatPrice(order.Price)
	}

	response, err := r.privateApi.AddOrder(api.Pair(order.Coin), order.Type.String(), order.OType.String(), order.Coin.FormatVolume(order.Volume), params)
	if err != nil {
		return nil, nil, fmt.Errorf("could not add order '%+v': %w", order, err)
	}

	return nil, response.TransactionIds, nil
}

func (r *Remote) CancelOrder(idx string) (*krakenapi.CancelOrderResponse, error) {
	return r.privateApi.CancelOrder(idx)
}

func (r *Remote) OpenPositions() (*coinapi.PositionBatch, error) {
	params := map[string]string{
		"docalcs": "true",
	}
	response, err := r.privateApi.OpenPositions(params)
	if err != nil {
		return nil, fmt.Errorf("could not get positions: %w", err)
	}

	if response == nil {
		return nil, fmt.Errorf("received invalid response: %v", response)
	}

	positionsResponse := *response
	if len(positionsResponse) == 0 {
		return &coinapi.PositionBatch{
			Positions: []coinapi.Position{},
			Index:     time.Now().UnixSecond,
		}, nil
	}

	positions := make([]coinapi.Position, len(positionsResponse))
	i := 0
	for k, pos := range *response {
		positions[i] = private.NewPosition(k, pos)
		i++
	}
	return &coinapi.PositionBatch{
		Positions: positions,
		Index:     time.Now().UnixSecond,
	}, nil
}

func (r *Remote) Close() error {
	return nil
}

type Hybrid struct {
	Remote *Remote
	local  map[string]storage.Persistence
	force  bool
}

// HybridAPI creates an API that will use the store
// if force is set to tru it will start producing empty trades
// otherwise it will try to use the underlying external client.
func HybridAPI(api *Remote, force bool) *Hybrid {
	return &Hybrid{
		Remote: api,
		local:  make(map[string]storage.Persistence),
		force:  force,
	}
}

const tradesMethod = "Trades"

func (h *Hybrid) Trades(coin coinapi.Coin, since int64) (*coinapi.TradeBatch, error) {

	if _, ok := h.local[tradesMethod]; !ok {
		store := storage.NewJsonBlob(tradesMethod, coin.String())
		h.local[tradesMethod] = store
	}

	log.Trace().
		Str("method", "Count").
		Str("coin", coin.String()).
		Int64("Since", since).
		Msg("calling local")

	key := storage.Key{
		Since: since,
		Pair:  coin.String(),
	}

	response := &krakenapi.TradesResponse{}
	err := h.local[tradesMethod].Load(key, response)
	if err == nil {
		return transform(api.Pair(coin), response)
	}

	// force would force us to use only the cache
	if !h.force {
		response, err := h.Remote.Trades(coin, since)
		if err != nil {
			return nil, fmt.Errorf("could not get trades: %w", err)
		}

		err = h.local[tradesMethod].Store(key, response)
		if err != nil {
			log.Err(err).Msg("ERROR:put")
		}

		return response, err
	}

	return nil, fmt.Errorf("could not get trades: %w", err)

}

func (h *Hybrid) Close() error {
	var err error
	//for pair, repo := range api.local {
	//	repoErr := repo.Close()
	//	log.Error().Err(repoErr).Str("pair", pair).Msg("could not close repository")
	//	if err == nil {
	//		err = repoErr
	//	}
	//}
	return err
}

type Custom struct {
	trades chan coinapi.Trade
}

// CustomAPI creates an API that will use the store
// if force is set to true it will start producing empty trades
// otherwise it will try to use the underlying external client.
func CustomAPI(trades chan coinapi.Trade) *Custom {
	return &Custom{
		trades: trades,
	}
}

func (c *Custom) Trades(pair string, since int64) (*krakenapi.TradesResponse, error) {
	trades := make([]krakenapi.TradeInfo, 0)
	for trade := range c.trades {
		trades = append(trades, krakenapi.TradeInfo{
			Price:       fmt.Sprintf("%f", trade.Price),
			PriceFloat:  trade.Price,
			Volume:      fmt.Sprintf("%f", trade.Volume),
			VolumeFloat: trade.Volume,
			Time:        trade.Time.UnixSecond,
			Buy:         trade.Type == coinapi.Buy,
			Sell:        trade.Type == coinapi.Sell,
		})
	}

	return &krakenapi.TradesResponse{
		Last:   since,
		Trades: trades,
	}, nil

}

func (c *Custom) Close() error {
	return nil
}

func (c *Custom) Balance() (*coinapi.AccountBalance, error) {
	panic("implement me")
}

func (c *Custom) OpenOrders() (*krakenapi.OpenOrdersResponse, error) {
	panic("implement me")
}

func (c *Custom) TradesHistory() (*krakenapi.TradesHistoryResponse, error) {
	panic("implement me")
}

func (c *Custom) Order(pair string, dir coinapi.Type, volume float64) (*krakenapi.AddOrderResponse, error) {
	panic("implement me")
}

func (c *Custom) AssetPairs() (*krakenapi.AssetPairsResponse, error) {
	panic("implement me")
}
