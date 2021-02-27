package local

import (
	"time"

	"github.com/drakos74/free-coin/internal/model"
)

// TrackedPosition is a wrapper for the position adding a timestamp of the position related event.
type TrackedPosition struct {
	Open     time.Time
	Close    time.Time
	Position model.Position
}
