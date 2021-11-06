package stats

import (
	"fmt"
	"math"
	"time"

	coinmath "github.com/drakos74/free-coin/internal/math"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

type OrderName string

const (
	NumericStrategy OrderName = "numeric"
	O10             OrderName = "O10"
	O2              OrderName = "O2"
)

type Signal struct {
	Coin     model.Coin    `json:"coin"`
	Duration time.Duration `json:"duration"`
	Segments int           `json:"segments"`
	Factor   float64       `json:"factor"`
	Density  float64       `json:"density"`
	Type     model.Type    `json:"type"`
	Price    float64       `json:"price"`
	Time     time.Time     `json:"time"`
}

func (s *Signal) Filter(threshold int) bool {
	f := math.Pow(10, float64(threshold))
	return s.Factor*f >= 1
}

// Window defines the window stats collector
type Window struct {
	W buffer.HistoryWindow `json:"window"`
	// TODO : see if we can remove the pointer here as well
	C *buffer.HMM `json:"hmm"`
}

// StaticWindow is a static representation of the window state.
// It s used for storing the window state.
type StaticWindow struct {
	W buffer.HistoryWindow `json:"window"`
	C buffer.HMM           `json:"hmm"`
}

func newWindow(key model.Key, cfg Config, store storage.Persistence) Window {
	// find out the max window size
	hmm := make([]buffer.HMMConfig, len(cfg.Model.Stats))
	var windowSize int
	for i, stat := range cfg.Model.Stats {
		ws := stat.LookAhead + stat.LookBack + 1
		if windowSize < ws {
			windowSize = ws
		}
		hmm[i] = buffer.HMMConfig{
			LookBack:  stat.LookBack,
			LookAhead: stat.LookAhead,
		}
	}
	// check if we have a reference to a stored instance
	if cfg.Model.Index > 0 {
		key.Index = cfg.Model.Index
		var window StaticWindow
		err := store.Load(processor.NewStateKey(Name, key), &window)
		log.Info().
			Err(err).
			Str("key", fmt.Sprintf("%+v", key)).
			Int64("index", cfg.Model.Index).
			Msg("loading previous state for HMM")
		if err != nil {
			return Window{
				W: buffer.NewHistoryWindow(cointime.ToMinutes(cfg.Duration), windowSize),
				C: buffer.HMMFromState(window.C),
			}
		}
	}
	log.Info().
		Str("key", fmt.Sprintf("%+v", key)).
		Int64("index", cfg.Model.Index).
		Msg("start new HMM")
	return Window{
		W: buffer.NewHistoryWindow(cointime.ToMinutes(cfg.Duration), windowSize),
		C: buffer.NewMultiHMM(hmm...),
	}
}

// Config defines the processor configuration
type Config struct {
	Name     string
	Duration int
	Model    Model
	Order    Order
	Notify   Notify
}

// ConfigBuilder is a struct to help build the configuration for the stats collector
type ConfigBuilder struct {
	config Config
}

// New creates a new config builder.
func New(name string, duration time.Duration) *ConfigBuilder {
	return &ConfigBuilder{
		config: Config{
			Name:     name,
			Duration: int(duration.Minutes()),
			Order:    Parse(Order{Name: O10}),
		},
	}
}

// Add adds a new stats processor.
func (c *ConfigBuilder) Add(prev, next int) *ConfigBuilder {
	c.config.Model.Stats = append(c.config.Model.Stats, Stats{
		LookBack:  prev,
		LookAhead: next,
	})
	return c
}

func (c *ConfigBuilder) O(o OrderName) *ConfigBuilder {
	order := Order{
		Name: o,
	}
	c.config.Order = Parse(order)
	return c
}

// Notify specifies the notification condition for current config
func (c *ConfigBuilder) Notify() *ConfigBuilder {
	c.config.Notify = Notify{Stats: true}
	return c
}

func (c *ConfigBuilder) Build() Config {
	return c.config
}

// Notify defines notification actions and limits
type Notify struct {
	Stats bool `json:"stats"`
}

// Model defines the stats conditions and instance.
type Model struct {
	Index int64   `json:"index"`
	Stats []Stats `json:"stats"`
}

// Stats defines the prediction target intervals.
type Stats struct {
	LookBack  int `json:"prev"`
	LookAhead int `json:"next"`
}

// Order defines the order of the aggregation logic for the stats
type Order struct {
	Name OrderName           `json:"name"`
	Exec func(f float64) int `json:"-"`
}

func Parse(o Order) Order {
	o.Exec = getOrderFunc(o.Name)
	return o
}

func getOrderFunc(order OrderName) func(f float64) int {
	switch order {
	case O10:
		return coinmath.IO10
	case O2:
		return coinmath.IO2
	}
	log.Error().Str("order", string(order)).Msg("unknown order func in config")
	return func(f float64) int {
		// very bad logic ... but what can we do as default ?
		return int(f)
	}
}
