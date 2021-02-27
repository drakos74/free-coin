package stats

import (
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type orderFunc func(f float64) int

type window struct {
	w *buffer.HistoryWindow
	c *buffer.HMM
}

type windowConfig struct {
	coin         model.Coin
	duration     time.Duration
	order        orderFunc
	historySizes int64
	counterSizes []buffer.HMMConfig
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
	}, nil
}

type statsCollector struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	lock    sync.RWMutex
	configs map[model.Coin]map[time.Duration]windowConfig
	windows map[sKey]window
}

func newStats(initialConfigs []Config) (*statsCollector, error) {
	stats := &statsCollector{
		lock:    sync.RWMutex{},
		configs: make(map[model.Coin]map[time.Duration]windowConfig),
		windows: make(map[sKey]window),
	}
	// add default initialConfigs to start with ...
	for _, dd := range initialConfigs {
		c := model.Coin(dd.Coin)

		if _, ok := stats.configs[c]; !ok {
			stats.configs[c] = make(map[time.Duration]windowConfig)
		}

		log.Info().
			Str("coin", dd.Coin).
			Str("intervals", fmt.Sprintf("%+v", dd.Targets)).
			Int("duration", dd.Duration).
			Msg("adding stats initialConfig")
		duration := time.Duration(dd.Duration) * time.Minute
		config, err := newWindowConfig(duration, dd)
		if err != nil {
			return nil, fmt.Errorf("could not init stats collector: %w", err)
		}

		if _, ok := stats.configs[c][duration]; ok {
			log.Error().Str("coin", string(c)).Int("duration", dd.Duration).Msg("duplicate stats config key")
			continue
		}
		// we can have only one config per coin per duration
		stats.configs[c][duration] = config
	}
	return stats, nil
}

//func (s *statsCollector) hasOrAddDuration(dd Config) bool {
//	s.lock.RLock()
//	defer s.lock.RUnlock()
//	if _, ok := s.configs[dd.Duration]; ok {
//		return true
//	}
//	s.configs[dd.Duration] = newWindowConfig(dd)
//	return false
//}

type sKey struct {
	duration time.Duration
	coin     model.Coin
}

func (s *statsCollector) start(k sKey) {
	s.lock.Lock()
	defer s.lock.Unlock()
	cfg := s.configs[k.coin][k.duration]
	if _, ok := s.windows[k]; !ok {
		s.windows[k] = window{
			w: buffer.NewHistoryWindow(cfg.duration, int(cfg.historySizes)),
			c: buffer.NewMultiHMM(cfg.counterSizes...),
		}
		log.Info().
			Str("counters", fmt.Sprintf("%+v", cfg.counterSizes)).
			Int64("size", cfg.historySizes).
			Float64("Duration", cfg.duration.Minutes()).
			Str("Coin", string(k.coin)).
			Msg("started stats processor")
	}
}

func (s *statsCollector) stop(k sKey) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.configs[k.coin], k.duration)
	delete(s.windows, k)
}

func (s *statsCollector) push(k sKey, trade *model.Trade) ([]interface{}, bool) {
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

func (s *statsCollector) add(k sKey, v string) (map[string]buffer.Prediction, buffer.Status) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.windows[k].c.Add(v, fmt.Sprintf("%dm", int(k.duration.Minutes())))
}
