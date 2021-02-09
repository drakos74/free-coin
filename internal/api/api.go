package api

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/internal/algo/model"
)

// TradeClient exposes the low level interface for interacting with a trade source.
type TradeClient interface {
	Trades(stop <-chan struct{}, coin model.Coin, stopExecution model.Condition) (model.TradeSource, error)
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
	Send(message *Message, trigger *Trigger) int
	// Reply replies to the user with a confirmation or an error.
	Reply(message *Message, err error)
}

// Message defines a message that should be sent to the user or group.
type Message struct {
	text  string
	reply int
}

// NewMessage creates a new message.
func NewMessage(txt string) *Message {
	return &Message{
		text: txt,
	}
}

// ReplyTo defines a message id that this message refers to.
func (m *Message) ReplyTo(msgID int) *Message {
	m.reply = msgID
	return m
}

// AddLine adds a line argument to the message.
func (m *Message) AddLine(txt string) *Message {
	m.text = fmt.Sprintf("%s\n%s", m.text, txt)
	return m
}

// Create creates a new messager implementation to send the current message.
func (m *Message) Create(constructor func(m Message) interface{}, obj interface{}) {
	// TODO : test and fix this.
	obj = constructor(*m)
}
