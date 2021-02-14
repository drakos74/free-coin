package api

import "fmt"

// Message defines a message that should be sent to the user or group.
type Message struct {
	Text  string
	Reply int
}

// NewMessage creates a new message.
func NewMessage(txt string) *Message {
	return &Message{
		Text: txt,
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
