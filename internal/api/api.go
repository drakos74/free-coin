package api

import (
	"context"
	"github.com/drakos74/free-coin/internal/algo/model"
	"time"
)

// TradeClient exposes the low level interface for interacting with a trade source.
type TradeClient interface {
	Trades(ctx context.Context, coin model.Coin, stopExecution model.StopExecution) model.TradeSource
	OpenPosition(position model.Position) error
	ClosePosition(position model.Position) error
}

// TriggerFunc defines an execution logic based on the command and options arguments.
type TriggerFunc func(command Command, options ...string) (string, error)

// Trigger wraps a trigger func into a re-usable object.
type Trigger struct {
	ID      string
	Default []string
	Exec    TriggerFunc
	Timeout time.Duration
}

// Command is the definitions of metadata for a command.
type Command struct {
	ID      int
	User    string
	Content string
}

// Interface defines an external interface for exchanging information and sharing control with the user(s)
type Interface interface {
	Run(ctx context.Context) error
	Listen(key, prefix string) <-chan Command
	Send(txt string, trigger *Trigger) (int, error)
	SendWithRef(txt string, ref int, trigger *Trigger) (int, error)
}
