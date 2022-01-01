package trader

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

// Key defines a trading entity key
type Key struct {
	Coin     model.Coin
	Duration time.Duration
}

func FromString(k string) Key {
	ss := strings.Split(k, "_")
	if len(ss) != 2 {
		panic(fmt.Sprintf("%s : invalid key", k))
	}
	m, err := strconv.Atoi(ss[1])
	if err != nil {
		panic(err.Error())
	}
	return Key{
		Coin:     model.Coin(ss[0]),
		Duration: time.Duration(m) * time.Minute,
	}
}

func (k Key) ToString() string {
	return fmt.Sprintf("%s_%.0f", k.Coin, k.Duration.Minutes())
}
