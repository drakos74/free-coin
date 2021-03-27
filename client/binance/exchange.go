package binance

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/adshao/go-binance/v2"
	"github.com/drakos74/free-coin/client/binance/model"
	coinmodel "github.com/drakos74/free-coin/internal/model"
)

const (
	key       = "BINANCE_KEY"
	secret    = "BINANCE_SECRET"
	extKey    = "EXT_BINANCE_KEY"
	extSecret = "EXT_BINANCE_SECRET"
)

// Exchange is a binance client wrapper
type Exchange struct {
	api       *binance.Client
	info      map[coinmodel.Coin]binance.Symbol
	converter model.Converter
}

// NewExchange creates a new binance client.
func NewExchange(option Option) *Exchange {
	k, s := ExchangeConfig(option)
	client := binance.NewClient(k, s)
	exchange := &Exchange{
		api:       client,
		converter: model.NewConverter(),
	}
	exchange.getInfo()
	return exchange
}

func (c *Exchange) getInfo() {
	if c.info != nil {
		return
	}
	info, err := c.api.NewExchangeInfoService().Do(context.Background())
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

func (c *Exchange) OpenPositions(ctx context.Context) (*coinmodel.PositionBatch, error) {
	return coinmodel.EmptyBatch(), nil
}

func (c *Exchange) OpenOrder(order coinmodel.TrackedOrder) (coinmodel.TrackedOrder, []string, error) {
	s, ok := c.info[order.Coin]
	if !ok {
		return order, nil, fmt.Errorf("could not find exchange info for %s [%d]", order.Coin, len(c.info))
	}
	orderResponse, err := c.api.NewCreateOrderService().
		Symbol(c.converter.Coin.Pair(order.Coin)).
		Side(c.converter.Type.From(order.Type)).
		Type(c.converter.OrderType.From(order.OType)).
		//TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(strconv.FormatFloat(order.Volume, 'f', s.BaseAssetPrecision, 64)).
		//Price(coinmodel.Price.Format(order.Coin, order.Volume)).
		Do(context.Background())
	log.Debug().Str("response", fmt.Sprintf("%+v", orderResponse)).Msg("order")
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
