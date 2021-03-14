package server

import "context"

// Status defines the status of the request
type Status int

const (
	// Start defines a request has started
	Start Status = iota + 1
	// Finish defines the finishing of a request
	Finish
)

// Control allows for controlling the flow of the action
type Control struct {
	Status    Status
	Type      string
	Interrupt bool
	Cancel    context.CancelFunc
	Reaction  chan struct{}
}

func NewController(s Status, t string) *Control {
	return &Control{
		Status:   s,
		Type:     t,
		Cancel:   func() {},
		Reaction: make(chan struct{}),
	}
}

// WithContext assigns a context to the action
func (a *Control) WithCancel(cancel context.CancelFunc) *Control {
	a.Cancel = cancel
	return a
}

// AllowInterrupt signifies that this action can be interrupted
func (a *Control) AllowInterrupt() *Control {
	a.Interrupt = true
	return a
}
