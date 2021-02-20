package local

import (
	"context"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/model"

	"github.com/drakos74/free-coin/internal/api"
)

// Client will retrieve trades from the given key value storage.
// It will be able to retrieve and store data from an upstream client,
// if the storage does not contain the requested data.
// furthermore it will mock the client behaviour in terms of the positions and orders locally.
// It can be used as a simulation environment for testing processors and business logic.
type Client struct {
	upstream    api.TradeClient
	persistence storage.Persistence
}

func (c Client) Trades(stop <-chan struct{}, coin model.Coin, stopExecution api.Condition) (model.TradeSource, error) {
	panic("implement me")
}

func (c Client) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	panic("implement me")
}

func (c Client) OpenPosition(position model.Position) error {
	panic("implement me")
}

func (c Client) ClosePosition(position model.Position) error {
	panic("implement me")
}
