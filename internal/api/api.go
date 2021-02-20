package api

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/internal/model"
)

type Index bool

// TODO : star simple now, but we can make it an int64 in the future ;)
const (
	Public  Index = false
	Private Index = true
)

// TradeClient exposes the low level interface for interacting with a trade source.
// TODO : split the trade retrieval and ordering logic.
type TradeClient interface {
	Trades(stop <-chan struct{}, coin model.Coin, stopExecution Condition) (model.TradeSource, error)
	OpenPositions(ctx context.Context) (*model.PositionBatch, error)
	OpenPosition(position model.Position) error
	ClosePosition(position model.Position) error
}

// UserInterface defines an external interface for exchanging information and sharing control with the user(s)
type UserInterface interface {
	// Run starts the user interface implementation and initialises any external connections.
	Run(ctx context.Context) error
	// Listen returns a channel of commands to the caller to interact with the user.
	// the caller needs to provide a unique subscription key.
	// additionally the caller can define a prefix to avoid being spammed with messages not relevant to them.
	Listen(key, prefix string) <-chan Command
	// Send sends a message to the user adn returns the message ID
	Send(channel Index, message *Message, trigger *Trigger) int
}

// Reply sends a reply message based on the given error to the user.
func Reply(private Index, user UserInterface, message *Message, err error) {
	if err != nil {
		message.AddLine(fmt.Sprintf("error:%s", err.Error()))
	}
	user.Send(private, message, nil)
}