package binance

import (
	"context"
	"fmt"
	"strconv"

	"github.com/drakos74/free-coin/internal/api"

	"github.com/drakos74/free-coin/internal/account"

	"github.com/rs/zerolog/log"

	"github.com/adshao/go-binance/v2"
	"github.com/drakos74/free-coin/client/binance/model"
	coinmodel "github.com/drakos74/free-coin/internal/model"
)

// Exchange is a binance client wrapper
type Exchange struct {
	api       exchange
	info      map[coinmodel.Coin]binance.Symbol
	converter model.Converter
}

// NewExchange creates a new binance client.
func NewExchange(user account.Name) *Exchange {
	exchange := &Exchange{
		api:       createAPI(user),
		converter: model.NewConverter(),
	}
	exchange.getInfo()
	return exchange
}

func (c *Exchange) getInfo() {
	if c.info != nil {
		return
	}
	info, err := c.api.GetExchangeInfo()
	if err != nil {
		log.Error().Str("exchange", "binance").Msg("could not get exchange info")
	}
	log.Info().
		Int("pairs", len(info.Symbols)).
		Str("exchange", "binance").
		Msg("exchange info")
	symbols := make(map[coinmodel.Coin]binance.Symbol)
	for _, s := range info.Symbols {
		coin := coinmodel.Coin(s.Symbol)
		symbols[coin] = s
	}
	c.info = symbols
}

func (c *Exchange) CoinInfo(coin coinmodel.Coin) binance.Symbol {
	return c.info[coin]
}

func (c *Exchange) CurrentPrice(ctx context.Context) (map[coinmodel.Coin]coinmodel.CurrentPrice, error) {
	prices, err := c.api.ListPrice()

	if err != nil {
		return nil, fmt.Errorf("could not get price list: %w", err)
	}
	priceMap := make(map[coinmodel.Coin]coinmodel.CurrentPrice)
	for _, price := range prices {
		coin := coinmodel.Coin(price.Symbol)
		p, err := strconv.ParseFloat(price.Price, 64)
		if err != nil {
			log.Error().Str("coin", price.Symbol).Err(err).Msg("could not parse price")
		}

		priceMap[coin] = coinmodel.CurrentPrice{
			Coin:  coin,
			Price: p,
		}
	}
	return priceMap, nil
}

func (c *Exchange) OpenPositions(ctx context.Context) (*coinmodel.PositionBatch, error) {
	return coinmodel.EmptyBatch(), nil
}

func (c *Exchange) OpenOrder(order coinmodel.TrackedOrder) (coinmodel.TrackedOrder, []string, error) {
	s, ok := c.info[order.Coin]
	if !ok {
		return order, nil, fmt.Errorf("could not find exchange info for %s [%d]", order.Coin, len(c.info))
	}

	lotSize, lotErr := model.ParseLOTSize(s.Filters)
	if lotErr != nil {
		log.Trace().Str("filters", fmt.Sprintf("%+v", s.Filters)).Msg("no lot size filter found")
	}

	// adjust the volume to comply with lot size filter
	order.Volume = lotSize.Adjust(order.Volume)
	volume := strconv.FormatFloat(order.Volume, 'f', s.BaseAssetPrecision, 64)

	log.Debug().
		Str("volume", volume).
		Str("order", fmt.Sprintf("%+v", order)).
		Msg("submit order")

	orderResponse, err := c.api.CreateOrder(c.converter, order, volume)
	log.Debug().Err(err).Str("response", fmt.Sprintf("%+v", orderResponse)).Msg("order")
	if err != nil {
		return order, nil, fmt.Errorf("could not complete order: %w", err)
	}

	price := 0.0
	count := 0.0
	for _, fill := range orderResponse.Fills {
		if fill != nil {
			p, err := strconv.ParseFloat(fill.Price, 64)
			if err != nil {
				count = 1.0
				break
			}
			q, err := strconv.ParseFloat(fill.Quantity, 64)
			if err != nil {
				count = 1.0
				break
			}
			price += q * p
			count += q
		}
	}
	order.Price = price / count
	return order, []string{orderResponse.ClientOrderID}, nil
}

func (c *Exchange) ClosePosition(position coinmodel.Position) error {
	order := coinmodel.NewOrder(position.Coin).
		Market().
		WithVolume(position.Volume).
		WithLeverage(coinmodel.L_5).
		WithType(position.Type.Inv()).
		Create()

	_, _, err := c.OpenOrder(coinmodel.TrackedOrder{
		Order: order,
	})
	return err
}

func (c *Exchange) Pairs(ctx context.Context) map[string]api.Pair {
	pairs := make(map[string]api.Pair)
	for k := range c.info {
		pairs[string(k)] = api.Pair{Coin: k}
	}
	return pairs
}

// Balance returns the account balance.
// if price map is empty it will try to retrieve it with the CurrentPrice API.
func (c *Exchange) Balance(ctx context.Context, priceMap map[coinmodel.Coin]coinmodel.CurrentPrice) (map[coinmodel.Coin]coinmodel.Balance, error) {
	balances := make(map[coinmodel.Coin]coinmodel.Balance)

	prices := priceMap
	if prices == nil {
		p, err := c.CurrentPrice(ctx)
		if err != nil {
			log.Error().Err(err).Msg("could not get price info")
			//return balance, fmt.Errorf("could not get price information: %w", err)
			prices = make(map[coinmodel.Coin]coinmodel.CurrentPrice)
		} else {
			prices = p
		}
	}

	account, err := c.api.GetAccount()

	if err != nil {
		return balances, fmt.Errorf("could not get balance: %w", err)
	}

	for _, b := range account.Balances {

		pair := fmt.Sprintf("%sUSDT", b.Asset)
		coin := coinmodel.Coin(pair)

		vol, err := strconv.ParseFloat(b.Free, 64)
		if err != nil {
			return balances, fmt.Errorf("could not parse volume for %s: %w", coin, err)
		}

		if vol == 0 {
			continue
		}

		locked, err := strconv.ParseFloat(b.Locked, 64)
		if err != nil {
			log.Error().Str("coin", string(coin)).Err(err).Msg("could not parse locked assets")
		}

		p, ok := prices[coin]
		if !ok {
			log.Error().Str("coin", string(coin)).Msg("could not find price")
		}

		balance := coinmodel.Balance{
			Coin:   coin,
			Volume: vol,
			Price:  p.Price,
			Locked: locked,
		}
		balances[coin] = balance
	}

	return balances, nil
}

type MarginExchange struct {
	pairs map[coinmodel.Coin]binance.MarginAllPair
	Exchange
}

func NewMarginExchange(user account.Name) *MarginExchange {
	exchange := &MarginExchange{
		pairs: make(map[coinmodel.Coin]binance.MarginAllPair),
		Exchange: Exchange{
			api:       createAPI(user),
			converter: model.NewConverter(),
		},
	}

	exchange.getInfo()
	exchange.getPairs()

	return exchange
}

func (e *MarginExchange) getPairs() {
	pairs, err := e.api.GetMarginPairs()
	if err != nil {
		log.Error().Err(err).Msg("could not get margin pairs")
	}
	for _, pair := range pairs {
		if pair != nil {
			coin := coinmodel.Coin(pair.Symbol)
			e.pairs[coin] = *pair
		}
	}
}

func (e *MarginExchange) OpenOrder(order coinmodel.TrackedOrder) (coinmodel.TrackedOrder, []string, error) {
	if _, ok := e.pairs[order.Coin]; !ok {
		return order, nil, fmt.Errorf("not a valid margin pair: %s", string(order.Coin))
	}
	log.Debug().
		Str("coin", string(order.Coin)).
		Str("pair", fmt.Sprintf("%+v", e.pairs[order.Coin])).
		Msg("trade on margin")
	order.Leverage = coinmodel.L_3
	return e.Exchange.OpenOrder(order)
}

// Balance returns the account balance.
// if price map is empty it will try to retrieve it with the CurrentPrice API.
func (e *MarginExchange) Balance(ctx context.Context, priceMap map[coinmodel.Coin]coinmodel.CurrentPrice) (map[coinmodel.Coin]coinmodel.Balance, error) {
	balances := make(map[coinmodel.Coin]coinmodel.Balance)

	prices := priceMap
	if prices == nil {
		p, err := e.CurrentPrice(ctx)
		if err != nil {
			log.Error().Err(err).Msg("could not get price info")
			//return balance, fmt.Errorf("could not get price information: %w", err)
			prices = make(map[coinmodel.Coin]coinmodel.CurrentPrice)
		} else {
			prices = p
		}
	}

	account, err := e.api.GetMarginAccount()
	if err != nil {
		return balances, fmt.Errorf("could not get balance: %w", err)
	}

	for _, b := range account.UserAssets {

		pair := fmt.Sprintf("%sUSDT", b.Asset)
		coin := coinmodel.Coin(pair)

		vol, err := strconv.ParseFloat(b.Free, 64)
		if err != nil {
			return balances, fmt.Errorf("could not parse volume for %s: %w", coin, err)
		}

		if vol == 0 {
			continue
		}

		locked, err := strconv.ParseFloat(b.Locked, 64)
		if err != nil {
			log.Error().Str("coin", string(coin)).Err(err).Msg("could not parse locked assets")
		}

		p, ok := prices[coin]
		if !ok {
			log.Error().Str("coin", string(coin)).Msg("could not find price")
		}

		balance := coinmodel.Balance{
			Coin:   coin,
			Volume: vol,
			Price:  p.Price,
			Locked: locked,
		}

		balances[coin] = balance

	}

	return balances, nil
}
