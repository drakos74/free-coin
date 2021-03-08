package model

import (
	"time"

	"github.com/google/uuid"
)

// TrackedPosition is a wrapper for the position adding a timestamp of the position related event.
type TrackedPosition struct {
	Open  time.Time `json:"open"`
	Close time.Time `json:"close"`
	Position
}

type TrackedPositions []TrackedPosition

// for sorting predictions
func (p TrackedPositions) Len() int           { return len(p) }
func (p TrackedPositions) Less(i, j int) bool { return p[i].Open.Before(p[j].Open) }
func (p TrackedPositions) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// PositionBatch is a batch of open positions.
type PositionBatch struct {
	Positions []Position
	Index     int64
}

// Position defines an open position details.
type Position struct {
	ID           string    `json:"id"`
	TxID         string    `json:"txId"`
	OrderID      string    `json:"order_id"`
	CID          string    `json:"cid"`
	OpenTime     time.Time `json:"open_time"`
	Coin         Coin      `json:"coin"`
	Type         Type      `json:"type"`
	OpenPrice    float64   `json:"open_price"`
	CurrentPrice float64   `json:"current_price"`
	Volume       float64   `json:"volume"`
	Cost         float64   `json:"cost"`
	Net          float64   `json:"net"`
	Fees         float64   `json:"fees"`
}

// NewPosition creates a new position.
// it inherits the coin, type and price of the current trade,
// but specifies its own quantity.
func NewPosition(trade Trade, volume float64) Position {
	return Position{
		ID:        uuid.New().String(),
		Coin:      trade.Coin,
		Type:      trade.Type,
		OpenPrice: trade.Price,
		Volume:    volume,
	}
}

// Value returns the value of the position and the profit or loss percentage.
func (p *Position) Value() (value, percent float64) {
	net := 0.0
	switch p.Type {
	case Buy:
		net = p.CurrentPrice - p.OpenPrice
	case Sell:
		net = p.OpenPrice - p.CurrentPrice
	}
	value = (net * p.Volume) - p.Fees
	return value, 100 * value / (p.OpenPrice * p.Volume)
}

// OpenPosition creates a position from a given order.
func OpenPosition(order Order) Position {
	return Position{
		ID:        uuid.New().String(),
		OrderID:   order.ID,
		Coin:      order.Coin,
		Type:      order.Type,
		Volume:    order.Volume,
		OpenPrice: order.Price,
	}
}
