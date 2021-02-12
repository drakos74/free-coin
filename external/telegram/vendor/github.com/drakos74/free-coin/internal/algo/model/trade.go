package model

import "time"

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

// Processor defines the processing model of input and output channels for trades.
// Each processor will trigger the next one, when pushing the trade to the output channel.
type Processor func(in <-chan Trade, out chan<- Trade) error

// Condition defines a boundary condition to stop execution based on the consumed trades.
type Condition func(trade Trade, numberOfTrades int) bool

func Counter(limit int) func(trade Trade, numberOfTrades int) bool {
	return func(trade Trade, numberOfTrades int) bool {
		return numberOfTrades > 0 && numberOfTrades >= limit
	}
}

func Until(time time.Time) func(trade Trade, numberOfTrades int) bool {
	return func(trade Trade, numberOfTrades int) bool {
		return trade.Time.After(time)
	}
}

func WhileHasTrades() func(trade Trade, numberOfTrades int) bool {
	return func(trade Trade, numberOfTrades int) bool {
		// TODO : find a better condition, for now, in combination with Cache(force = true) this does it's job.
		return trade.Price == 0
	}
}

func NonStop(trade Trade, numberOfTrades int) bool {
	return false
}
