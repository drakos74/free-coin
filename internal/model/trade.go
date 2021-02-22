package model

import (
	"time"
)

// TradeSource is a channel for receiving and sending trades.
type TradeSource chan *Trade

// Trade represents a trade object with all necessary details.
type Trade struct {
	ID     string    `json:"id"`
	Coin   Coin      `json:"coin"`
	Price  float64   `json:"Value"`
	Volume float64   `json:"Volume"`
	Time   time.Time `json:"time"`
	Type   Type      `json:"Type"`
	Active bool      `json:"active"`
	Live   bool      `json:"live"`
}
