package trade

import "github.com/drakos74/free-coin/internal/model"

type OpenConfig struct {
	Coin                 model.Coin
	SampleThreshold      int
	ProbabilityThreshold float64
	Volume               float64
	Strategies           []TradingStrategy
}

func newOpenConfig(c model.Coin, vol float64) OpenConfig {
	return OpenConfig{
		Coin:   c,
		Volume: vol,
		Strategies: []TradingStrategy{
			simpleStrategy,
		},
	}
}
func (c OpenConfig) withProbability(p float64) OpenConfig {
	c.ProbabilityThreshold = p
	return c
}

func (c OpenConfig) withSample(s int) OpenConfig {
	c.SampleThreshold = s
	return c
}

func (c OpenConfig) contains(vv []string) model.Type {
	for _, strategy := range c.Strategies {
		if t := strategy.exec(vv); t != model.NoType {
			return t
		}
	}
	return model.NoType
}

type TradingStrategy struct {
	name string
	exec func(vv []string) model.Type
}
