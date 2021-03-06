package stats

import (
	"fmt"
	"sync"
	"time"

	cointime "github.com/drakos74/free-coin/internal/time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
)

type window struct {
	w *buffer.HistoryWindow
	c *buffer.HMM
}

func newWindow(cfg processor.Config) *window {
	// find out the max window size
	hmm := make([]buffer.HMMConfig, len(cfg.Stats))
	var windowSize int
	for i, stat := range cfg.Stats {
		ws := stat.LookAhead + stat.LookBack + 1
		if windowSize < ws {
			windowSize = ws
		}
		hmm[i] = buffer.HMMConfig{
			LookBack:  stat.LookBack,
			LookAhead: stat.LookAhead,
		}
	}
	return &window{
		w: buffer.NewHistoryWindow(cointime.ToMinutes(cfg.Duration), windowSize),
		c: buffer.NewMultiHMM(hmm...),
	}
}

type statsCollector struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	logger  storage.Registry
	lock    sync.RWMutex
	configs map[model.Coin]map[time.Duration]processor.Config
	windows map[processor.Key]*window
}

func newStats(registry storage.Registry, configs map[model.Coin]map[time.Duration]processor.Config) (*statsCollector, error) {

	windows := make(map[processor.Key]*window)

	for c, dConfig := range configs {
		for d, cfg := range dConfig {
			k := processor.Key{
				Coin:     c,
				Duration: d,
			}
			windows[k] = newWindow(cfg)
		}
	}

	stats := &statsCollector{
		logger:  registry,
		lock:    sync.RWMutex{},
		windows: windows,
		configs: configs,
	}
	return stats, nil
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

// add adds the new value to the model and returns the predictions taking the current sample into account
// It will return one prediction per lookback/lookahead configuration of the HMM
func (s *statsCollector) add(k processor.Key, v string) (map[buffer.Sequence]buffer.Predictions, buffer.Status) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.windows[k].c.Add(v, fmt.Sprintf("%dm", int(k.Duration.Minutes())))
}
