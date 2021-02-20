package api

import (
	"time"

	"github.com/drakos74/free-coin/internal/model"
)

// Processor defines the processing model of input and output channels for trades.
// Each processor will trigger the next one, when pushing the trade to the output channel.
// TODO : add load and save functionality for processors interface to allow saving and loading state.
type Processor func(in <-chan *model.Trade, out chan<- *model.Trade)

// Condition defines a boundary condition to stop execution based on the consumed trades.
type Condition func(trade *model.Trade, numberOfTrades int) bool

func Counter(limit int) func(trade *model.Trade, numberOfTrades int) bool {
	return func(trade *model.Trade, numberOfTrades int) bool {
		return numberOfTrades > 0 && numberOfTrades >= limit
	}
}

func Until(time time.Time) func(trade *model.Trade, numberOfTrades int) bool {
	return func(trade *model.Trade, numberOfTrades int) bool {
		return trade.Time.After(time)
	}
}

func WhileHasTrades() func(trade model.Trade, numberOfTrades int) bool {
	return func(trade model.Trade, numberOfTrades int) bool {
		// TODO : find a better condition, for now, in combination with Cache(force = true) this does it's job.
		return trade.Price == 0
	}
}

func NonStop(trade *model.Trade, numberOfTrades int) bool {
	return false
}
