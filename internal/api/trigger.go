package api

import (
	"time"

	"github.com/google/uuid"
)

// TriggerFunc defines an execution logic based on the command and options arguments.
type TriggerFunc func(command Command, options ...string) (string, error)

// Trigger wraps a trigger func into a re-usable object.
type Trigger struct {
	ID          string
	Default     []string
	Description string
	Exec        TriggerFunc
	Timeout     time.Duration
}

// NewTrigger creates a new trigger.
func NewTrigger(f TriggerFunc) *Trigger {
	return &Trigger{
		ID:   uuid.New().String(),
		Exec: f,
	}
}

// WithID allows to specify a custom ID for the trigger.
func (t *Trigger) WithID(id string) *Trigger {
	t.ID = id
	return t
}

// WithID allows to specify a custom ID for the trigger.
func (t *Trigger) WithDescription(desc string) *Trigger {
	t.Description = desc
	return t
}

// WithTimeout allows the user to specify a custom timeout for the trigger.
func (t *Trigger) WithTimeout(timeout time.Duration) *Trigger {
	t.Timeout = timeout
	return t
}

// WithDefaults specifies default argument for the auto-callback.
func (t *Trigger) WithDefaults(defaults ...string) *Trigger {
	t.Default = defaults
	return t
}
