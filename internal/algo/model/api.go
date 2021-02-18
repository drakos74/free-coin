package model

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
)

// TradeClient exposes the low level interface for interacting with a trade source.
type TradeClient interface {
	Trades(stop <-chan struct{}, coin api.Coin, stopExecution api.Condition) (api.TradeSource, error)
	OpenPositions(ctx context.Context) (*api.PositionBatch, error)
	OpenPosition(position api.Position) error
	ClosePosition(position api.Position) error
}

// UserInterface defines an external interface for exchanging information and sharing control with the user(s)
type UserInterface interface {
	// Run starts the user interface implementation and initialises any external connections.
	Run(ctx context.Context) error
	// Listen returns a channel of commands to the caller to interact with the user.
	// the caller needs to provide a unique subscription key.
	// additionally the caller can define a prefix to avoid being spammed with messages not relevant to them.
	Listen(key, prefix string) <-chan api.Command
	// Send sends a message to the user adn returns the message ID
	Send(message *api.Message, trigger *api.Trigger) int
}

// Reply sends a reply message based on the given error to the user.
func Reply(user UserInterface, message *api.Message, err error) {
	if err != nil {
		message.AddLine(fmt.Sprintf("error:%s", err.Error()))
	}
	user.Send(message, nil)
}
