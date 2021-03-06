package processor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

const (
	NumericStrategy = "numeric"
	O10             = "O10"
	O2              = "O2"
)

// Config defines the configuration for the MultiStats processor.
type Config struct {
	Duration   int        `json:"duration"`
	Order      Order      `json:"order"`
	Notify     Notify     `json:"notify"`
	Stats      []Stats    `json:"stats"`
	Strategies []Strategy `json:"strategies"`
}

// Order defines the order of the aggregation logic for the stats
type Order struct {
	Name string              `json:"name"`
	Exec func(f float64) int `json:"-"`
}

// Notify defines notification actions and limits
type Notify struct {
	Stats bool `json:"stats"`
}

// Stats defines the prediction target intervals.
type Stats struct {
	LookBack  int `json:"prev"`
	LookAhead int `json:"next"`
}

// Strategy defines the trading strategy for generating trade signals
type Strategy struct {
	// Open defines the open conditions for the trade
	Open Open `json:"open"`
	// Close defines the closing conditions for the trade
	Close Close `json:"close"`
	// Name defines the name of the strategy . NOTE : it needs to be one of the available options
	Name string `json:"name"`
	// Probability defines the minimum probability of predictions for the strategy tot take effect.
	Probability float64 `json:"probability"`
	// Sample defines the minimum prediction sample for the strategy to take effect
	Sample int `json:"sample"`
	// Threshold defines the prediction result numeric threshold for the strategy to take effect.
	Threshold float64 `json:"threshold"`
	// Factor defines the factor applied to confidence of the prediction the amount of opening the order.
	Factor float64 `json:"factor"`
}

// Open is the configuration for opening positions.
type Open struct {
	Value float64 `json:"value"`
	Limit float64 `json:"limit"`
}

// Close defines the conditions for closing the position.
type Close struct {
	Instant bool  `json:"instant"`
	Profit  Setup `json:"profit"`
	Loss    Setup `json:"loss"`
}

// Setup defines a threshold set up for closing positions in profit or loss conditions.
type Setup struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Trail float64 `json:"trail"`
	High  float64 `json:"-"`
}

const path = "infra/config"

// LoadDefaults loads the default config from the predefined location.
// It will pass over any default config - if it exists - to each coin,
// if its missing from the coin.
func LoadDefaults(coins map[string]model.Coin) map[model.Coin]map[time.Duration]Config {
	var configs map[string][]Config
	MustLoad("config", &configs)

	defaultConfig, hasDefault := configs[""]

	cfg := make(map[model.Coin]map[time.Duration]Config)
	for coin, c := range coins {
		cfg[c] = make(map[time.Duration]Config)
		// generate config for each coin from the default
		if _, ok := configs[coin]; !ok {
			if hasDefault {
				for _, def := range defaultConfig {
					timeDuration := cointime.ToMinutes(def.Duration)
					cfg[c][timeDuration] = Parse(def)
					log.Info().
						Str("coin", coin).
						Float64("min", timeDuration.Minutes()).
						Msg("init default config")
				}
			}
		} else {
			// check if we need to pass over anything from the default
			for _, config := range configs[coin] {
				timeDuration := cointime.ToMinutes(config.Duration)
				cfg[c][timeDuration] = Parse(config)
				log.Info().
					Str("coin", coin).
					Float64("min", timeDuration.Minutes()).
					Msg("init concrete config")
			}
			// check if we miss something from the defaults
			for _, def := range defaultConfig {
				timeDuration := cointime.ToMinutes(def.Duration)
				if _, hasConfig := cfg[c][timeDuration]; !hasConfig {
					cfg[c][timeDuration] = Parse(def)
					log.Info().
						Str("coin", coin).
						Float64("min", timeDuration.Minutes()).
						Msg("migrated from default config")
				}
			}
		}
	}

	return cfg
}

// MustLoad loads the config for the given key
func MustLoad(key string, v interface{}) []byte {

	b, err := ioutil.ReadFile(fmt.Sprintf("%s/%s.json", path, key))
	if err != nil {
		panic(fmt.Sprintf("could not load config for %s: %s", key, err.Error()))
	}

	err = json.Unmarshal(b, v)
	if err != nil {
		panic(fmt.Sprintf("could not unmarshal the config for %s: %s", key, err.Error()))
	}

	log.Info().Str("processor", key).Msg("loaded config")

	return b

}

func Parse(config Config) Config {
	config.Order.Exec = getOrderFunc(config.Order.Name)
	return config
}

func getOrderFunc(order string) func(f float64) int {
	switch order {
	case O10:
		return coinmath.IO10
	case O2:
		return coinmath.IO2
	}
	log.Error().Str("order", order).Msg("unknown order func in config")
	return func(f float64) int {
		// very bad logic ... but what can we do as default ?
		return int(f)
	}
}

// GetAny returns the first strategy we can find
// this is highly unsafe, but is as a backup,
// in case we were not able to reason about a strategy and we dont want to stop processing actions.
func GetAny(mm map[time.Duration]Config, cid string) Strategy {
	if cid != "" {
		// try to find the config from the given startegy
		_, duration, strategy, err := DeCorrelate(cid)
		if err == nil {
			if dConfig, ok := mm[duration]; ok {
				for _, str := range dConfig.Strategies {
					if str.Name == strategy {
						log.Info().
							Str("cid", cid).
							Str("strategy", fmt.Sprintf("%+v", str)).
							Msg("tracked strategy")
						return str
					}
				}
			}
		}
	}
	log.Warn().
		Str("cid", cid).
		Msg("using random strategy ... ")
	for _, m := range mm {
		if len(m.Strategies) > 0 {
			return m.Strategies[0]
		}
	}
	return Strategy{}
}

const delimiter = "-"

// Correlate concatenates the given information into a string.
func Correlate(coin model.Coin, duration time.Duration, strategy string) string {
	return fmt.Sprintf("%s%s%d%s%s", coin, delimiter, int(duration.Minutes()), delimiter, strategy)
}

// DeCorrelate splits the given string into the correlated parts.
func DeCorrelate(cid string) (coin model.Coin, duration time.Duration, strategy string, err error) {
	parts := strings.Split(cid, delimiter)
	if len(parts) != 3 {
		err = fmt.Errorf("could not de-correlate '%s'", cid)
		return
	}
	coin = model.Coin(parts[0])
	strategy = parts[2]
	d, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		err = fmt.Errorf("could not de-correlate '%s': %w", cid, err)
		return
	}
	duration = time.Duration(d) * time.Minute
	return
}
