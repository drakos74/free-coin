package external

import (
	"fmt"
	"sync"

	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const storagePath = "external-positions"

type tracker struct {
	positions map[string]model.Position
	storage   storage.Persistence
	lock      *sync.RWMutex
}

func newTracker(shard storage.Shard) (*tracker, error) {
	st, err := shard(storagePath)
	if err != nil {
		return nil, fmt.Errorf("could not init storage: %w", err)
	}
	positions := make(map[string]model.Position)
	err = st.Load(stKey(), &positions)
	log.Info().Err(err).Int("num", len(positions)).Msg("loaded positions")
	return &tracker{
		positions: positions,
		storage:   st,
		lock:      new(sync.RWMutex),
	}, nil
}

func (t *tracker) getAll() map[string]model.Position {
	t.lock.RLock()
	defer t.lock.RUnlock()
	positions := make(map[string]model.Position)
	for k, p := range t.positions {
		positions[k] = p
	}
	return positions
}

func (t *tracker) check(key string) (model.Position, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if p, ok := t.positions[key]; ok {
		return p, true
	}
	return model.Position{}, false
}

func (t *tracker) add(key string, order model.TrackedOrder, close bool) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	if close {
		if _, ok := t.positions[key]; !ok {
			return fmt.Errorf("cannot find posiiton to close for key: %s", key)
		}
		delete(t.positions, key)
		return nil
	}
	// we need to be careful here and add the position ...
	position := model.OpenPosition(order)
	if p, ok := t.positions[key]; ok {
		if position.Coin != p.Coin {
			return fmt.Errorf("different coin found for key: %s [%s vs %s]", key, p.Coin, position.Coin)
		}
		if position.Type != p.Type {
			return fmt.Errorf("different type found for key: %s [%s vs %s]", key, p.Type.String(), position.Type.String())
		}
		position.Volume += p.Volume
		log.Warn().
			Str("key", key).
			Float64("from", p.Volume).
			Float64("to", position.Volume).
			Msg("extending position")
	}
	t.positions[key] = position
	return t.storage.Store(stKey(), t.positions)
}

func stKey() storage.Key {
	return storage.Key{
		Pair:  "all",
		Label: ProcessorName,
	}
}
