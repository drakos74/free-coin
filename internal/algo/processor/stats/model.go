package stats

import (
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/storage"
)

func NewStateKey(label string) storage.Key {
	return storage.Key{
		Pair:  "stats",
		Label: label,
	}
}

type Window struct {
	w *buffer.HistoryWindow `json:"window"`
	c *buffer.HMM           `json:"hmm"`
}
