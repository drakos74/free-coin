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
	state   storage.Persistence
	lock    sync.RWMutex
	configs map[model.Coin]map[time.Duration]Config
	windows map[model.Key]Window
}

func newStats(shard storage.Shard, configs map[model.Coin]map[time.Duration]Config) (*statsCollector, error) {

	windows := make(map[model.Key]Window)

	state, err := shard(Name)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		state = storage.NewVoidStorage()
	}

	for c, dConfig := range configs {
		for d, cfg := range dConfig {
			k := model.Key{
				Coin:     c,
				Duration: d,
				Strategy: cfg.Name,
			}
			// TODO : check if we can load the window (?)
			key := model.NewKey(c, d, cfg.Name)
			windows[k] = newWindow(key, cfg, state)
		}
	}

	stats := &statsCollector{
		state:   state,
		lock:    sync.RWMutex{},
		windows: windows,
		configs: configs,
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

func (s *statsCollector) push(k model.Key, trade *model.Trade) ([]interface{}, map[int][]float64, float64, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if b, ok := s.windows[k].W.Push(trade.Time, trade.Price, trade.Price*trade.Volume); ok {
		poly := make(map[int][]float64)
		poly2, err := s.windows[k].W.Polynomial(0, func(b buffer.TimeWindowView) float64 {
			return b.Ratio
		}, 2)
		if err != nil {
			log.Debug().Int("degree", 2).Msg("could not fit polynomial")
		}
		poly[2] = poly2
		poly3, err := s.windows[k].W.Polynomial(0, func(b buffer.TimeWindowView) float64 {
			return b.Ratio
		}, 3)
		poly[3] = poly3
		if err != nil {
			log.Debug().Int("degree", 3).Msg("could not fit polynomial")
		}
		buckets := s.windows[k].W.Get(func(bucket buffer.TimeBucket) interface{} {
			priceView := buffer.NewView(bucket, 0)
			volumeView := buffer.NewView(bucket, 1)
			return windowView{
				price:  priceView,
				volume: volumeView,
			}
		})
		return buckets, poly, float64(b.Bucket.Values().Stats()[0].Count()), ok
	}
	return nil, nil, 0, false
}

// add adds the new value to the model and returns the predictions taking the current sample into account
// It will return one prediction per lookback/lookahead configuration of the HMM
func (s *statsCollector) add(k model.Key, v string) (map[buffer.Sequence]buffer.Predictions, buffer.Status) {
	s.lock.Lock()
	defer s.lock.Unlock()
	predictions, status := s.windows[k].C.Add(v, fmt.Sprintf("%dm", int(k.Duration.Minutes())))
	// dont store anything for now ...until we fix the structs and pointers
	storage.Store(s.state, processor.NewStateKey(Name, k), StaticWindow{
		W: s.windows[k].W,
		C: *s.windows[k].C,
	})
	return predictions, status
}
