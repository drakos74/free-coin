package local

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

type MockExchange struct {
	openPositionCalls int
	positions         []*model.PositionBatch
}

// NewMockExchange creates a new mock exchange implementation.
func NewMockExchange() *MockExchange {
	return &MockExchange{
		positions: make([]*model.PositionBatch, 0),
	}
}

func (e *MockExchange) AddOpenPositionResponse(positionBatch ...*model.PositionBatch) {
	e.positions = append(e.positions, positionBatch...)
}

func (e *MockExchange) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	if e.openPositionCalls < len(e.positions) {
		currenPositions := e.positions[e.openPositionCalls]
		e.openPositionCalls++
		return currenPositions, nil
	}
	return nil, fmt.Errorf("no open positions present at %d of %d", e.openPositionCalls, len(e.positions))
}

func (e *MockExchange) OpenOrder(order *model.TrackedOrder) (*model.TrackedOrder, []string, error) {
	panic("implement me")
}

func (e *MockExchange) ClosePosition(position *model.Position) error {
	panic("implement me")
}

func (e MockExchange) Balance(ctx context.Context, priceMap map[model.Coin]model.CurrentPrice) (map[model.Coin]model.Balance, error) {
	panic("implement me")
}

func (e *MockExchange) Pairs(ctx context.Context) map[string]api.Pair {
	panic("implement me")
}

func (e *MockExchange) CurrentPrice(ctx context.Context) (map[model.Coin]model.CurrentPrice, error) {
	panic("implement me")
}
