package signal

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const storagePath = "signals"

type config struct {
	multiplier float64
	base       float64
}

func newConfig(b float64) config {
	return config{
		multiplier: 1.0,
		base:       b,
	}
}

func (c config) value() float64 {
	return c.multiplier * c.base
}

func (c config) String() string {
	return fmt.Sprintf("%.2f * %.2f -> %.2f", c.base, c.multiplier, c.value())
}

type trader struct {
	client    api.Exchange
	settings  map[model.Coin]map[time.Duration]Settings
	positions map[string]model.Position
	storage   storage.Persistence
	account   string
	running   bool
	config    map[model.Coin]config
	lock      *sync.RWMutex
}

func newTrader(id string, client api.Exchange, shard storage.Shard, settings map[model.Coin]map[time.Duration]Settings) (*trader, error) {
	st, err := shard(path.Join(storagePath, id))
	if err != nil {
		return nil, fmt.Errorf("could not init storage: %w", err)
	}
	positions := make(map[string]model.Position)
	err = st.Load(stKey(), &positions)
	log.Info().Err(err).
		Str("account", id).
		Int("num", len(positions)).Msg("loaded positions")
	return &trader{
		client:    client,
		settings:  settings,
		positions: positions,
		storage:   st,
		account:   id,
		running:   true,
		config:    make(map[model.Coin]config),
		lock:      new(sync.RWMutex),
	}, nil
}

func (t *trader) getAll(ctx context.Context) ([]string, map[string]model.Position, map[model.Coin]model.CurrentPrice) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	prices, err := t.client.CurrentPrice(ctx)
	if err != nil {
		log.Error().Err(err).
			Str("account", t.account).
			Msg("could not get current prices")
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
func (t *trader) check(key string, coin model.Coin) (model.Position, bool, []model.Position) {
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

func (t *trader) add(key string, order model.TrackedOrder, close string) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	if close != "" {
		if _, ok := t.positions[key]; !ok {
			return fmt.Errorf("cannot find posiiton to close for key: %s", key)
		}
		delete(t.positions, key)
		return t.storage.Store(stKey(), t.positions)
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
			Str("account", t.account).
			Str("key", key).
			Float64("from", p.Volume).
			Float64("to", position.Volume).
			Msg("extending position")
	}
	t.positions[key] = position
	return t.storage.Store(stKey(), t.positions)
}

func (t *trader) parseConfig(c model.Coin, b float64) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if _, ok := t.config[c]; !ok {
		t.config[c] = newConfig(b)
	}
}

func (t *trader) updateConfig(multiplier float64, match func(c model.Coin) bool) error {
	if multiplier <= 0.0 {
		return fmt.Errorf("cannot update config for negative multipler %f", multiplier)
	}

	var m bool

	newConfig := make(map[model.Coin]config)
	for c, cfg := range t.config {
		if match(c) {
			cfg.multiplier = multiplier
			m = true
		}
		newConfig[c] = cfg
	}

	if !m {
		return fmt.Errorf("could not match any coin")
	}
	t.config = newConfig

	return nil
}

func stKey() storage.Key {
	return storage.Key{
		Pair:  "all",
		Label: ProcessorName,
	}
}
