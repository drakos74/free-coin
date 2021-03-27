package binance

import (
	"context"
	"fmt"

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
	converter model.Converter
}

// NewExchange creates a new binance client.
func NewExchange(option Option) *Exchange {
	k, s := ExchangeConfig(option)
	client := binance.NewClient(k, s)
	return &Exchange{
		api:       client,
		converter: model.NewConverter(),
	}
}

func (c *Exchange) OpenPositions(ctx context.Context) (*coinmodel.PositionBatch, error) {
	return coinmodel.EmptyBatch(), nil
}

func (c *Exchange) OpenOrder(order coinmodel.TrackedOrder) ([]string, error) {
	orderResponse, err := c.api.NewCreateOrderService().
		Symbol(c.converter.Coin.Pair(order.Coin)).
		Side(c.converter.Type.From(order.Type)).
		Type(c.converter.OrderType.From(order.OType)).
		//TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(coinmodel.Volume.Format(order.Coin, order.Volume)).
		Price(coinmodel.Price.Format(order.Coin, order.Volume)).
		Do(context.Background())
	log.Debug().Str("response", fmt.Sprintf("%+v", orderResponse)).Msg("order")
	if err != nil {
		return nil, fmt.Errorf("could not complete order: %w", err)
	}
	return []string{orderResponse.ClientOrderID}, nil
}

func (c *Exchange) ClosePosition(position coinmodel.Position) error {
	order := coinmodel.NewOrder(position.Coin).
		Market().
		WithVolume(position.Volume).
		WithLeverage(coinmodel.L_5).
		WithType(position.Type.Inv()).
		Create()

	_, err := c.OpenOrder(coinmodel.TrackedOrder{
		Order: order,
	})
	return err
}
