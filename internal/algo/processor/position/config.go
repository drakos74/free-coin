package position

import (
	"github.com/drakos74/free-coin/infra/config"
)

type Config struct {
	Coin   string `json:"prev"`
	Profit Setup  `json:"profit"`
	Loss   Setup  `json:"loss"`
}

type Setup struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Trail float64 `json:"trail"`
	High  float64 `json:"-"`
}

func loadDefaults() []Config {
	var configs []Config
	config.MustLoad(ProcessorName, &configs)
	return configs
}
