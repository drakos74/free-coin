package trader

import (
	"time"

	"github.com/drakos74/free-coin/internal/model"
)

// Settings are the settings for this processor.
type Settings struct {
	Open float64
}

// Config is the designated config for the trader processor.
type Config struct {
	ReportingPort int
	Settings      map[model.Coin]map[time.Duration]Settings
}
