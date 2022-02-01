package trader

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/model"
)

const (
	minSize = 50
)

type State struct {
	MinSize   int                       `json:"min_size"`
	Running   bool                      `json:"running"`
	Positions map[string]model.Position `json:"positions"`
}

type Settings struct {
	OpenValue  float64
	TakeProfit float64
	StopLoss   float64
}

type config struct {
	multiplier float64
	base       float64
}

func newConfig(b float64) config {
	return config{
		multiplier: 1.0,
		base:       b,
	}
}

func (c config) value() float64 {
	return c.multiplier * c.base
}

func (c config) String() string {
	return fmt.Sprintf("%.2f * %.2f -> %.2f", c.base, c.multiplier, c.value())
}

func FromString(k string) model.Key {
	ss := strings.Split(k, model.Delimiter)
	if len(ss) < 2 {
		panic(fmt.Sprintf("%s : invalid key", k))
	}
	m, err := strconv.Atoi(ss[1])
	if err != nil {
		panic(err.Error())
	}
	return model.Key{
		Coin:     model.Coin(ss[0]),
		Duration: time.Duration(m) * time.Minute,
	}
}

type Reason string

const (
	SignalReason       Reason = "signal"
	StopLossReason     Reason = "stop-loss"
	TakeProfitReason   Reason = "take-profit"
	VoidReasonIgnore   Reason = "void-ignore"
	VoidReasonConflict Reason = "void-conflict"
	VoidReasonType     Reason = "void-type"
	VoidReasonClose    Reason = "void-close"
)

// Event defines a trading action for reference and debugging.
type Event struct {
	Key    model.Key  `json:"key"`
	Time   time.Time  `json:"time"`
	Type   model.Type `json:"type"`
	Price  float64    `json:"price"`
	Value  float64    `json:"value"`
	Reason Reason     `json:"reason"`
}

// Log defines a collection of events and actions.
type Log struct {
	registry storage.Registry       `json:"-"`
	Events   map[model.Coin][]Event `json:"events"`
}

// NewEventLog create a new event and action log.
func NewEventLog(registry storage.Registry) *Log {
	return &Log{
		registry: registry,
		Events:   make(map[model.Coin][]Event),
	}
}

func (l *Log) append(event Event) error {
	if _, ok := l.Events[event.Key.Coin]; !ok {
		l.Events[event.Key.Coin] = make([]Event, 0)
	}
	log.Debug().Str("event", fmt.Sprintf("%+v", event)).Msg("adding event")
	l.Events[event.Key.Coin] = append(l.Events[event.Key.Coin], event)
	return l.registry.Add(storage.K{
		Pair:  string(event.Key.Coin),
		Label: event.Key.ToString(),
	}, event)
}
