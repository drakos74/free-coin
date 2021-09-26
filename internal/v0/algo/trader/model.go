package trader

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/model"
)

const (
	StoragePath   = "portfolio"
	intervalKey   = "interval"
	accumulateKey = "accumulate"
	minSize       = 20.0
)

// stKey creates the storage key for the trading processor.
func stKey() storage.Key {
	return storage.Key{
		Pair:  "all",
		Hash:  1,
		Label: ProcessorName,
	}
}

// compoundKey defines the concatenation logic to produce the unique user key identifier.
func (t *trader) compoundKey(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, t.account)
}

// config defines the configuration for the trader.
type config struct {
	multiplier float64
	base       float64
}

// newConfig creates a new config with the given base order size.
func newConfig(m float64) config {
	return config{
		base: m,
	}
}

// value returns the order size value for the config.
func (c config) value() float64 {
	return c.multiplier * c.base
}

// String prints the config details.
func (c config) String() string {
	return fmt.Sprintf("%.2f * %.2f -> %.2f", c.base, c.multiplier, c.value())
}

// State defines the state of the trader.
// This struct is used to save the configuration in order for the process to keep track of it's state.
type State struct {
	MinSize   int                       `json:"min_size"`
	Running   bool                      `json:"running"`
	Positions map[string]model.Position `json:"positions"`
}

type Signal interface {
	Time() time.Time
}

type Order struct {
	Signal Signal             `json:"signal"`
	Order  model.TrackedOrder `json:"order"`
	Errors map[string]string  `json:"errors"`
}

type Orders []Order

// for sorting predictions
func (o Orders) Len() int           { return len(o) }
func (o Orders) Less(i, j int) bool { return o[i].Signal.Time().Before(o[j].Signal.Time()) }
func (o Orders) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }

// QueryData represents the query data details.
type QueryData struct {
	Interval time.Duration
	Acc      bool
}
