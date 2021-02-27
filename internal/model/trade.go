package model

import (
	"time"
)

// Signal is a generic envelop for packing generic objects and passing them from process to process.
type Signal struct {
	Type  string
	Value interface{}
}

// VoidSignal is a void signal
var VoidSignal = Signal{}

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
	Signal Signal    `json:"-"`
}
