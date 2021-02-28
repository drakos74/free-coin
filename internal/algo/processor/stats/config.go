package stats

import (
	"fmt"

	"github.com/drakos74/free-coin/infra/config"
	coinmath "github.com/drakos74/free-coin/internal/math"
)

// Config defines the configuration for the MultiStats processor.
type Config struct {
	Coin      string   `json:"coin"`
	Duration  int      `json:"duration"`
	Order     string   `json:"order"`
	Intervals int      `json:"intervals"`
	Targets   []Target `json:"targets"`
}

// Target defines the prediction target intervals.
type Target struct {
	LookBack  int `json:"prev"`
	LookAhead int `json:"next"`
}

func loadDefaults() []Config {
	var configs []Config
	config.MustLoad(ProcessorName, &configs)
	return configs
}

func getOrderFunc(order string) (func(f float64) int, error) {
	switch order {
	case "O10":
		return coinmath.IO10, nil
	case "O2":
		return coinmath.IO2, nil
	}
	return nil, fmt.Errorf("unknown order function: %s", order)
}
