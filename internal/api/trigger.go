package api

import (
	"time"

	"github.com/google/uuid"
)

// ConsumerKey is the internal consumer key for indexing and managing consumers.
type ConsumerKey struct {
	ID     string
	Key    string
	Prefix string
}

// TriggerFunc defines an execution logic based on the command and options arguments.
type TriggerFunc func(command Command) (string, error)

// Trigger wraps a trigger func into a re-usable object.
type Trigger struct {
	ID          string
	Key         ConsumerKey
	Default     []string
	Description string
	Timeout     time.Duration
}

// NewTrigger creates a new trigger.
func NewTrigger(key ConsumerKey) *Trigger {
	id := key.ID
	if id == "" {
		id = uuid.New().String()
	}
	return &Trigger{
		ID:  id,
		Key: key,
	}
}

// WithID allows to specify a custom ID for the trigger.
func (t *Trigger) WithID(id string) *Trigger {
	t.ID = id
	return t
}

// WithDescription allows to specify a custom description for the trigger.
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
