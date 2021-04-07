package external

import (
	"context"
	"fmt"
	"path"
	"sync"

	"github.com/drakos74/free-coin/internal/api"

	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const storagePath = "signals"

type tracker struct {
	client    api.Exchange
	positions map[string]model.Position
	storage   storage.Persistence
	user      string
	running   bool
	lock      *sync.RWMutex
}

func newTracker(id string, client api.Exchange, shard storage.Shard) (*tracker, error) {
	st, err := shard(path.Join(storagePath, id))
	if err != nil {
		return nil, fmt.Errorf("could not init storage: %w", err)
	}
	positions := make(map[string]model.Position)
	err = st.Load(stKey(id), &positions)
	log.Info().Err(err).Int("num", len(positions)).Msg("loaded positions")
	return &tracker{
		client:    client,
		positions: positions,
		storage:   st,
		user:      id,
		running:   true,
		lock:      new(sync.RWMutex),
	}, nil
}

func (t *tracker) getAll(ctx context.Context) ([]string, map[string]model.Position, map[model.Coin]model.CurrentPrice) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	prices, err := t.client.CurrentPrice(ctx)
	if err != nil {
		log.Error().Err(err).Msg("could not get current prices")
		prices = make(map[model.Coin]model.CurrentPrice)
	}

	positions := make(map[string]model.Position)
	keys := make([]string, 0)
	for k, p := range t.positions {
		// check the current price
		if cp, ok := prices[p.Coin]; ok {
			p.CurrentPrice = cp.Price
		}
		positions[k] = p
		keys = append(keys, k)
	}
	return keys, positions, prices
}

// TODO : make this for now ... to have clear pairs
func (t *tracker) check(key string, coin model.Coin) (model.Position, bool, []model.Position) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if p, ok := t.positions[key]; ok {
		return p, true, []model.Position{}
	}
	// TODO : remove this at some point.
	// We want ... for now to ... basically avoid closing with the same coin but different key
	positions := make([]model.Position, 0)
	for _, p := range t.positions {
		if p.Coin == coin {
			positions = append(positions, p)
		}
	}
	return model.Position{}, false, positions
}

func (t *tracker) add(key string, order model.TrackedOrder, close string) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	if close != "" {
		if _, ok := t.positions[key]; !ok {
			return fmt.Errorf("cannot find posiiton to close for key: %s", key)
		}
		delete(t.positions, key)
		return t.storage.Store(stKey(t.user), t.positions)
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
	return t.storage.Store(stKey(t.user), t.positions)
}

func stKey(id string) storage.Key {
	return storage.Key{
		Pair:  "all",
		Label: path.Join(ProcessorName, id),
	}
}
