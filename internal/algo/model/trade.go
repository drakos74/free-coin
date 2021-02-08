package model

import "github.com/drakos74/free-coin/internal/time"

// TradeSource is a channel for receiving and sending trades.
type TradeSource chan Trade

// Trade represents a trade object with all necessary details.
type Trade struct {
	ID     string                 `json:"id"`
	Coin   Coin                   `json:"coin"`
	Price  float64                `json:"Price"`
	Volume float64                `json:"Volume"`
	Time   time.Time              `json:"time"`
	Type   Type                   `json:"Type"`
	Active bool                   `json:"active"`
	Meta   map[string]interface{} `json:"-"`
}

// Transform defines the processing model of input and output channels for trades.
// Each processor will trigger the next one, when pushing the trade to the output channel.
type Transform func(in <-chan Trade, out chan<- Trade) error

// StopExecution defines a boundary condition to stop execution based on the consumed trades.
type StopExecution func(trade Trade, numberOfTrades int) bool
