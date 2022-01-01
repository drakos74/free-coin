package api

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/internal/model"
)

// ExchangeName defines the name of the exchange
type ExchangeName string

// Index is the identifier for the user communication channel
type Index string

// StrategyProcessor defines a processing logic for a strategy
type StrategyProcessor func(u User, e Exchange) Processor

// Query is the trades query object.
type Query struct {
	Coin  model.Coin
	Index string
}

// Pair defines a coin trading pair.
type Pair struct {
	Coin model.Coin
}

// Client exposes the low level interface for interacting with a trade source.
type Client interface {
	Trades(process <-chan Signal) (model.TradeSource, error)
}

// Exchange allows interaction with the exchange for submitting and closing positions and trades.
type Exchange interface {
	OpenPositions(ctx context.Context) (*model.PositionBatch, error)
	OpenOrder(order *model.TrackedOrder) (*model.TrackedOrder, []string, error)
	Balance(ctx context.Context, priceMap map[model.Coin]model.CurrentPrice) (map[model.Coin]model.Balance, error)
	Pairs(ctx context.Context) map[string]Pair
	CurrentPrice(ctx context.Context) (map[model.Coin]model.CurrentPrice, error)
}

// User defines an external interface for exchanging information and sharing control with the user(s)
type User interface {
	// Run starts the user interface implementation and initialises any external connections.
	Run(ctx context.Context) error
	// Listen returns a channel of commands to the caller to interact with the user.
	// the caller needs to provide a unique subscription key.
	// additionally the caller can define a prefix to avoid being spammed with messages not relevant to them.
	Listen(key, prefix string) <-chan Command
	// Send sends a message to the user adn returns the message ID
	Send(channel Index, message *Message, trigger *Trigger) int
	// AddUser adds the given chatID for the specified user name
	AddUser(channel Index, user string, chatID int64) error
}

// Reply sends a reply message based on the given error to the user.
func Reply(private Index, user User, message *Message, err error) {
	if err != nil {
		message.AddLine(fmt.Sprintf("error:%s", err.Error()))
	}
	user.Send(private, message, nil)
}
