package api

import (
	"fmt"
	"time"
)

// Message defines a message that should be sent to the user or group.
type Message struct {
	Text  string
	Reply int
	Time  time.Time
}

// NewMessage creates a new message.
func NewMessage(txt string) *Message {
	return &Message{
		Text: txt,
		Time: time.Now(),
	}
}

// ReplyTo defines a message id that this message refers to.
func (m *Message) ReplyTo(msgID int) *Message {
	m.Reply = msgID
	return m
}

// AddLine adds a line argument to the message.
func (m *Message) AddLine(txt string) *Message {
	m.Text = fmt.Sprintf("%s\n%s", m.Text, txt)
	return m
}

// ReferenceTime adds a reference time to the message.
// This is especially useful for back-testing adn debugging.
func (m *Message) ReferenceTime(t time.Time) *Message {
	m.Time = t
	return m
}
