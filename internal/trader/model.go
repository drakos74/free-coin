package trader

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

// Action defines a trading action for reference and debugging.
type Action struct {
	Key    model.Key  `json:"key"`
	Time   time.Time  `json:"time"`
	Type   model.Type `json:"type"`
	Price  float64    `json:"price"`
	Value  float64    `json:"value"`
	Reason Reason     `json:"reason"`
}

// Log defines a collection of events and actions.
type Log struct {
	Actions map[model.Coin][]Action `json:"actions"`
}

// NewEventLog create a new event and action log.
func NewEventLog() *Log {
	return &Log{Actions: make(map[model.Coin][]Action)}
}

func (l *Log) append(action Action) {
	if _, ok := l.Actions[action.Key.Coin]; !ok {
		l.Actions[action.Key.Coin] = make([]Action, 0)
	}
	log.Debug().Str("action", fmt.Sprintf("%+v", action)).Msg("adding action")
	l.Actions[action.Key.Coin] = append(l.Actions[action.Key.Coin], action)
}
