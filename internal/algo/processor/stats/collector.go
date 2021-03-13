package stats

import (
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

type statsCollector struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	registry storage.Registry
	state    storage.Persistence
	lock     sync.RWMutex
	configs  map[model.Coin]map[time.Duration]processor.Config
	windows  map[model.Key]Window
}

func newStats(shard storage.Shard, registry storage.Registry, configs map[model.Coin]map[time.Duration]processor.Config) (*statsCollector, error) {

	windows := make(map[model.Key]Window)

	for c, dConfig := range configs {
		for d, cfg := range dConfig {
			k := model.Key{
				Coin:     c,
				Duration: d,
				Strategy: cfg.Strategy.Name,
			}
			// TODO : check if we can load the window (?)
			windows[k] = newWindow(cfg)
		}
	}

	state, err := shard(ProcessorName)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		state = storage.NewVoidStorage()
	}
	stats := &statsCollector{
		registry: registry,
		state:    state,
		lock:     sync.RWMutex{},
		windows:  windows,
		configs:  configs,
	}
	return stats, nil
}

// TODO : re-enable user interaction to start and stop stats collectors
//func (s *statsCollector) stop(k model.Key) {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//	delete(s.configs[k.Coin], k.Duration)
//	delete(s.windows, k)
//}

func (s *statsCollector) push(k model.Key, trade *model.Trade) ([]interface{}, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.windows[k].W.Push(trade.Time, trade.Price, trade.Price*trade.Volume); ok {
		buckets := s.windows[k].W.Get(func(bucket interface{}) interface{} {
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
func (s *statsCollector) add(k model.Key, v string) (map[buffer.Sequence]buffer.Predictions, buffer.Status) {
	s.lock.Lock()
	defer s.lock.Unlock()
	predictions, status := s.windows[k].C.Add(v, fmt.Sprintf("%dm", int(k.Duration.Minutes())))
	// dont store anything for now ...until we fix the structs and pointers
	storage.Store(s.state, NewStateKey(k.ToString()), StaticWindow{
		W: s.windows[k].W,
		C: *s.windows[k].C,
	})
	return predictions, status
}
