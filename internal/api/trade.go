package api

import (
	"time"

	"github.com/google/uuid"

	"github.com/drakos74/free-coin/internal/model"
)

// Processor defines the processing model of input and output channels for trades.
// Each processor will trigger the next one, when pushing the trade to the output channel.
// TODO : add load and save functionality for processors interface to allow saving and loading state.
type Processor func(in <-chan *model.TradeSignal, out chan<- *model.TradeSignal)

// Block allows 2 processes to sync
type Block struct {
	// Action block.Signal <- api.Signal{}
	Action chan Signal
	// ReAction	<-block.ReAction
	ReAction chan Signal
}

func NewBlock() Block {
	return Block{
		Action:   make(chan Signal),
		ReAction: make(chan Signal),
	}
}

// Signal is a generic struct used to trigger actions on other processes.
// it can hold metadata information , but for now we leave it empty.
type Signal struct {
	Name    string
	ID      string
	Coin    model.Coin
	Content interface{}
	Time    time.Time
}

// NewSignal creates a new action with the given name.
func NewSignal(name string) *Signal {
	return &Signal{
		Name: name,
		Time: time.Now(),
		ID:   uuid.New().String(),
	}
}

// Create returns an immutable instance of the action
func (a *Signal) Create() Signal {
	return *a
}

// ForCoin assigns a coin to the action
func (a *Signal) ForCoin(coin model.Coin) *Signal {
	a.Coin = coin
	return a
}

// WithID assigns an id to the action
func (a *Signal) WithID(id string) *Signal {
	a.ID = id
	return a
}

// WithContent adds content to the action.
// This would indicate an actionable event.
// The content should be de-coded by using the the name of the action.
func (a *Signal) WithContent(s interface{}) *Signal {
	a.Content = s
	return a
}

// Condition defines a boundary condition to stop execution based on the consumed trades.
type Condition func(trade *model.TradeSignal, numberOfTrades int) bool

func Counter(limit int) func(trade *model.TradeSignal, numberOfTrades int) bool {
	return func(trade *model.TradeSignal, numberOfTrades int) bool {
		return numberOfTrades > 0 && numberOfTrades >= limit
	}
}

func Until(time time.Time) func(trade *model.TradeSignal, numberOfTrades int) bool {
	return func(trade *model.TradeSignal, numberOfTrades int) bool {
		return trade.Meta.Time.After(time)
	}
}

func NonStop(trade *model.TradeSignal, numberOfTrades int) bool {
	return false
}
