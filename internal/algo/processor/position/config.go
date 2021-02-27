package position

import (
	"github.com/drakos74/free-coin/infra/config"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Coin   string `json:"prev"`
	Profit Setup  `json:"profit"`
	Loss   Setup  `json:"loss"`
}

type Setup struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Trail bool    `json:"trail"`
	High  float64 `json:"-"`
}

func (tp *tradePositions) getConfiguration(coin model.Coin) Config {
	// get it first from the predefined ones
	for _, config := range tp.configs {
		if model.Coin(config.Coin) == coin {
			return config
		}
	}
	log.Debug().Str("coin", string(coin)).Msg("load config from default")

	var cfg []Config
	config.MustLoad(ProcessorName, &cfg)
	for _, config := range cfg {
		if model.Coin(config.Coin) == coin {
			return config
		}
	}
	return cfg[0]
}
