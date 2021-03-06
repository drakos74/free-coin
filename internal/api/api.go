package api

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/internal/model"
)

type ExchangeName string

type Index string

// Query is the trades query object.
type Query struct {
	Coin  model.Coin
	Index string
}

type Pair struct {
	Coin model.Coin
}

// Client exposes the low level interface for interacting with a trade source.
// TODO : split the trade retrieval and ordering logic.
type Client interface {
	Trades(process <-chan Action, query Query) (model.TradeSource, error)
}

// Exchange allows interaction with the exchange for submitting and closing positions and trades.
type Exchange interface {
	OpenPositions(ctx context.Context) (*model.PositionBatch, error)
	OpenOrder(order model.TrackedOrder) (model.TrackedOrder, []string, error)
	ClosePosition(position model.Position) error
	CurrentPrice(ctx context.Context) (map[model.Coin]model.CurrentPrice, error)
	Balance(ctx context.Context, priceMap map[model.Coin]model.CurrentPrice) (map[model.Coin]model.Balance, error)
	Pairs(ctx context.Context) map[string]Pair
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
