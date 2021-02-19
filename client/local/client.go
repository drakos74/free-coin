package local

import (
	"context"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/model"

	"github.com/drakos74/free-coin/internal/api"
)

// Client will retrieve trades from the given key value storage.
// It will be able to retrieve and store data from an upstream client,
// if the storage does not contain the requested data.
// furthermore it will mock the client behaviour in terms of the positions and orders locally.
// It can be used as a simulation environment for testing processors and business logic.
type Client struct {
	upstream    model.TradeClient
	persistence storage.Persistence
}

func (c Client) Trades(stop <-chan struct{}, coin api.Coin, stopExecution api.Condition) (api.TradeSource, error) {
	panic("implement me")
}

func (c Client) OpenPositions(ctx context.Context) (*api.PositionBatch, error) {
	panic("implement me")
}

func (c Client) OpenPosition(position api.Position) error {
	panic("implement me")
}

func (c Client) ClosePosition(position api.Position) error {
	panic("implement me")
}
