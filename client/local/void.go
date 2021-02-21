package local

import (
	"context"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

type VoidClient struct {
}

func Void() api.TradeClient {
	return &VoidClient{}
}

func (v VoidClient) Trades(stop <-chan struct{}, coin model.Coin, stopExecution api.Condition) (model.TradeSource, error) {
	panic("implement me")
}

func (v VoidClient) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	panic("implement me")
}

func (v VoidClient) OpenPosition(position model.Position) error {
	panic("implement me")
}

func (v VoidClient) ClosePosition(position model.Position) error {
	panic("implement me")
}
