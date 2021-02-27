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
	duration     time.Duration
	order        orderFunc
	historySizes int64
	counterSizes []buffer.HMMConfig
}

func newWindowConfig(duration time.Duration, config MultiStatsConfig) (windowConfig, error) {
	// check the max history Duration we need to apply the given config
	var size int
	hmmConfigs := make([]buffer.HMMConfig, len(config.Targets))
	for _, target := range config.Targets {
		max := target.LookAhead + target.LookBack + 1
		if max > size {
			size = max
		}
		hmmConfigs = append(hmmConfigs, buffer.HMMConfig{
			LookBack:  target.LookBack,
			LookAhead: target.LookAhead,
		})
	}
	orderFunction, err := getOrderFunc(config.Order)
	if err != nil {
		return windowConfig{}, fmt.Errorf("could not get order func: '%s'", config.Order)
	}
	return windowConfig{
		duration:     duration,
		historySizes: int64(size),
		counterSizes: hmmConfigs,
		order:        orderFunction,
	}, nil
}

type statsCollector struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	lock    sync.RWMutex
	configs map[time.Duration]windowConfig
	windows map[time.Duration]map[model.Coin]window
}

func newStats(configs []MultiStatsConfig) (*statsCollector, error) {
	stats := &statsCollector{
		lock:    sync.RWMutex{},
		configs: make(map[time.Duration]windowConfig),
		windows: make(map[time.Duration]map[model.Coin]window),
	}
	// add default configs to start with ...
	for _, dd := range configs {
		log.Info().
			Str("intervals", fmt.Sprintf("%+v", dd.Targets)).
			Int("min", dd.Duration).
			Msg("added stats configs")
		duration := time.Duration(dd.Duration) * time.Minute
		config, err := newWindowConfig(duration, dd)
		if err != nil {
			return nil, fmt.Errorf("could not init stats collector: %w", err)
		}
		stats.configs[duration] = config
		stats.windows[duration] = make(map[model.Coin]window)
	}
	return stats, nil
}

//func (s *statsCollector) hasOrAddDuration(dd MultiStatsConfig) bool {
//	s.lock.RLock()
//	defer s.lock.RUnlock()
//	if _, ok := s.configs[dd.Duration]; ok {
//		return true
//	}
//	s.configs[dd.Duration] = newWindowConfig(dd)
//	return false
//}

func (s *statsCollector) start(dd time.Duration, coin model.Coin) {
	s.lock.Lock()
	defer s.lock.Unlock()
	cfg := s.configs[dd]
	if _, ok := s.windows[dd][coin]; !ok {
		s.windows[dd][coin] = window{
			w: buffer.NewHistoryWindow(cfg.duration, int(cfg.historySizes)),
			c: buffer.NewMultiHMM(cfg.counterSizes...),
		}
		log.Info().
			Str("counters", fmt.Sprintf("%+v", cfg.counterSizes)).
			Int64("size", cfg.historySizes).
			Float64("Duration", cfg.duration.Minutes()).
			Str("Coin", string(coin)).
			Msg("started stats processor")
	}
}

func (s *statsCollector) stop(dd time.Duration) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.configs, dd)
	delete(s.windows, dd)
}

func (s *statsCollector) push(dd time.Duration, trade *model.Trade) ([]interface{}, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.windows[dd][trade.Coin].w.Push(trade.Time, trade.Price, trade.Price*trade.Volume); ok {
		buckets := s.windows[dd][trade.Coin].w.Get(func(bucket interface{}) interface{} {
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

func (s *statsCollector) add(dd time.Duration, coin model.Coin, v string) (map[string]buffer.Prediction, buffer.Status) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.windows[dd][coin].c.Add(v, fmt.Sprintf("%dm", int(dd.Minutes())))
}
