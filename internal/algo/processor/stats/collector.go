package stats

import (
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type orderFunc func(f float64) int

type window struct {
	w *buffer.HistoryWindow
	c *buffer.HMM
}

func newWindow(cfg windowConfig) *window {
	return &window{
		w: buffer.NewHistoryWindow(cfg.duration, int(cfg.historySizes)),
		c: buffer.NewMultiHMM(cfg.counterSizes...),
	}
}

type windowConfig struct {
	coin         model.Coin
	duration     time.Duration
	order        orderFunc
	historySizes int64
	counterSizes []buffer.HMMConfig
	notify       bool
}

func newWindowConfig(duration time.Duration, config Config) (windowConfig, error) {
	// check the max history Duration we need to apply the given config
	var size int
	hmmConfigs := make([]buffer.HMMConfig, len(config.Targets))
	for i, target := range config.Targets {
		max := target.LookAhead + target.LookBack + 1
		if max > size {
			size = max
		}
		hmmConfigs[i] = buffer.HMMConfig{
			LookBack:  target.LookBack,
			LookAhead: target.LookAhead,
		}
	}
	orderFunction, err := getOrderFunc(config.Order)
	if err != nil {
		return windowConfig{}, fmt.Errorf("could not get order func: '%s'", config.Order)
	}
	return windowConfig{
		coin:         model.Coin(config.Coin),
		duration:     duration,
		historySizes: int64(size),
		counterSizes: hmmConfigs,
		order:        orderFunction,
		notify:       config.Notify,
	}, nil
}

type statsCollector struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	execID         int64
	logger         storage.Persistence
	lock           sync.RWMutex
	initialConfigs []Config
	configs        map[model.Coin]map[time.Duration]windowConfig
	windows        map[processor.Key]*window
}

func newStats(execID int64, initialConfigs []Config) (*statsCollector, error) {
	stats := &statsCollector{
		execID:         execID,
		logger:         json.NewLogger(ProcessorName),
		lock:           sync.RWMutex{},
		initialConfigs: initialConfigs,
		configs:        make(map[model.Coin]map[time.Duration]windowConfig),
		windows:        make(map[processor.Key]*window),
	}
	// add default initialConfigs to start with ...
	for _, cfg := range initialConfigs {
		c := model.Coin(cfg.Coin)
		d := time.Duration(cfg.Duration) * time.Minute
		k := processor.NewKey(c, d)
		if _, ok := stats.configs[c]; !ok {
			stats.configs[c] = make(map[time.Duration]windowConfig)
		}
		config, err := newWindowConfig(d, cfg)
		if err != nil {
			return nil, fmt.Errorf("could not init stats collector for %v: %w", k, err)
		}
		if _, ok := stats.configs[c][d]; ok {
			// we can have only one cfg per coin per d
			return nil, fmt.Errorf("duplicate stats cfg key for %v", k)
		}
		stats.configs[c][d] = config
		log.Info().
			Str("coin", cfg.Coin).
			Int("d", cfg.Duration).
			//Str("config", fmt.Sprintf("%+v", config)).
			Msg("adding stats initialConfig")
		if _, ok := stats.windows[k]; !ok {
			stats.windows[k] = newWindow(config)
		}
	}
	return stats, nil
}

// init initialises the config if it s not there of the coin
func (s *statsCollector) init(coin model.Coin) {
	// ma ke sure when the config map exists , also the window key is there
	if _, ok := s.configs[coin]; !ok {
		s.configs[coin] = make(map[time.Duration]windowConfig)
		if cfgs, ok := s.configs[""]; ok {
			for d, cfg := range cfgs {
				cfg.coin = coin
				s.configs[coin][d] = cfg
				k := processor.NewKey(coin, d)
				if _, ok := s.windows[k]; !ok {
					log.Info().
						Str("coin", string(coin)).
						Int("d", int(d.Minutes())).
						//Str("config", fmt.Sprintf("%+v", cfg)).
						Msg("adding stats initialConfig")
					s.windows[k] = newWindow(cfg)
				}
				log.Warn().Str("coin", string(coin)).Msg("mutated default config")
			}
		}
	}
}

// TODO : re-enable user interaction to start and stop stats collectors
//func (s *statsCollector) stop(k processor.Key) {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//	delete(s.configs[k.Coin], k.Duration)
//	delete(s.windows, k)
//}

func (s *statsCollector) push(k processor.Key, trade *model.Trade) ([]interface{}, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.windows[k].w.Push(trade.Time, trade.Price, trade.Price*trade.Volume); ok {
		buckets := s.windows[k].w.Get(func(bucket interface{}) interface{} {
			// it's a history window , so we expect to have history buckets inside
			if b, ok := bucket.(buffer.TimeBucket); ok {
				// get the 'zerowth' stats element, as we are only assing the Price a few lines above,
				// there is nothing more to retrieve from the bucket.
				priceView := buffer.NewView(b, 0)
				volumeView := buffer.NewView(b, 1)
				return windowView{
					price:  priceView,
					volume: volumeView,
				}
			}
			// TODO : this will break things a lot if we missed something ... 😅
			return nil
		})
		return buckets, ok
	}
	return nil, false
}

func (s *statsCollector) add(k processor.Key, v string) (map[string]buffer.Predictions, buffer.Status) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.windows[k].c.Add(v, fmt.Sprintf("%dm", int(k.Duration.Minutes())))
}
