package trade

import "github.com/drakos74/free-coin/internal/model"

type OpenConfig struct {
	MinSample      int
	MinProbability float64
	Value          float64
	Strategy       TradingStrategy
}

func (c OpenConfig) contains(vv []string) model.Type {
	return c.Strategy.exec(vv)
}

type TradingStrategy struct {
	name string
	exec func(vv []string) model.Type
}

func getVolume(price float64, value float64) float64 {
	return value / price
}
