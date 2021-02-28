package trade

import (
	"math"
	"strconv"

	"github.com/drakos74/free-coin/infra/config"

	"github.com/drakos74/free-coin/internal/model"
)

const (
	NumericStrategy = "numeric"
)

type Config struct {
	Coin       string     `json:"coin"`
	Open       Open       `json:"open"`
	Strategies []Strategy `json:"strategies"`
}

type Open struct {
	Value float64 `json:"value"`
}

type Strategy struct {
	Name        string  `json:"name"`
	Target      int     `json:"target"`
	Probability float64 `json:"probability"`
	Sample      int     `json:"sample"`
	Threshold   float64 `json:"threshold"`
}

func loadDefaults() []Config {
	var configs []Config
	config.MustLoad(ProcessorName, &configs)
	return configs
}

func getStrategy(name string, threshold float64) TradingStrategy {
	switch name {
	case NumericStrategy:
		return TradingStrategy{
			name: name,
			exec: func(vv []string) model.Type {
				t := model.NoType
				value := 0.0
				s := 0.0
				// make it simple if we have one prediction
				l := len(vv)
				for w, v := range vv {
					i, err := strconv.ParseFloat(v, 64)
					if err != nil {
						return t
					}
					g := float64(l-w) * i
					value += g
					s++
				}
				x := value / s
				t = model.SignedType(x)
				if math.Abs(x) >= threshold {
					return t
				}
				return model.NoType
			},
		}
	}
	return TradingStrategy{
		name: "void",
		exec: func(vv []string) model.Type {
			return model.NoType
		},
	}
}
