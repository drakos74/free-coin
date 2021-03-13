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
	W *buffer.HistoryWindow `json:"window"`
	C *buffer.HMM           `json:"hmm"`
}
