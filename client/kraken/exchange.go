package kraken

import (
	"context"
	"fmt"
	"os"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/client/kraken/model"
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
)

// Exchange implements the exchange interface for kraken.
type Exchange struct {
	Api  *RemoteExchange
	info map[coinmodel.Coin]krakenapi.AssetPairInfo
}

// NewExchange creates a new exchange client.
func NewExchange(ctx context.Context) api.Exchange {
	exchange := &Exchange{
		Api: &RemoteExchange{
			converter: model.NewConverter(),
			private:   krakenapi.New(os.Getenv(key), os.Getenv(secret)),
		},
	}
	exchange.Api.getInfo()
	client := exchange
	return client
}

func (e *Exchange) ClosePosition(position coinmodel.Position) error {
	order := coinmodel.NewOrder(position.Coin).
		Market().
		WithVolume(position.Volume).
		WithLeverage(coinmodel.L_5).
		WithType(position.Type.Inv()).
		Create()

	_, _, err := e.Api.Order(order)
	if err != nil {
		return fmt.Errorf("could not close position: %w", err)
	}
	return nil
}

func (e *Exchange) OpenOrder(order coinmodel.TrackedOrder) (coinmodel.TrackedOrder, []string, error) {
	_, txids, err := e.Api.Order(order.Order)
	if err != nil {
		return order, nil, fmt.Errorf("could not open order: %w", err)
	}
	return order, txids, nil
}

func (e *Exchange) CurrentPrice(ctx context.Context) (map[coinmodel.Coin]coinmodel.CurrentPrice, error) {
	return make(map[coinmodel.Coin]coinmodel.CurrentPrice), nil
}

// TODO : make sure we reterieve all the positions.
// If we have too many, the response might not have them all
func (e *Exchange) OpenPositions(ctx context.Context) (*coinmodel.PositionBatch, error) {
	params := map[string]string{
		"docalcs": "true",
	}
	response, err := e.Api.private.OpenPositions(params)
	if err != nil {
		return nil, fmt.Errorf("could not get positions: %w", err)
	}

	if response == nil {
		return nil, fmt.Errorf("received invalid response: %v", response)
	}

	positionsResponse := *response
	if len(positionsResponse) == 0 {
		return &coinmodel.PositionBatch{
			Positions: []coinmodel.Position{},
			Index:     time.Now().Unix(),
		}, nil
	}

	positions := make([]coinmodel.Position, len(positionsResponse))
	i := 0
	for k, pos := range *response {
		positions[i] = e.Api.newPosition(k, pos)
		i++
	}
	return &coinmodel.PositionBatch{
		Positions: positions,
		Index:     time.Now().Unix(),
	}, nil
}

func (e *Exchange) Balance(ctx context.Context, priceMap map[coinmodel.Coin]coinmodel.CurrentPrice) (map[coinmodel.Coin]coinmodel.Balance, error) {
	// TODO :
	return make(map[coinmodel.Coin]coinmodel.Balance), nil
}
