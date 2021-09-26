package trader

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

type trader struct {
	client    api.Exchange
	settings  map[model.Coin]map[time.Duration]Settings
	positions map[string]model.Position
	storage   storage.Persistence
	account   string
	running   bool
	minSize   int
	config    map[model.Coin]config
	lock      *sync.RWMutex
}

// TODO : track portfolio and notify accordingly ...

func newTrader(id string, client api.Exchange, shard storage.Shard, settings map[model.Coin]map[time.Duration]Settings) (*trader, error) {
	st, err := shard(path.Join(StoragePath, id))
	if err != nil {
		return nil, fmt.Errorf("could not init storage: %w", err)
	}
	positions := make(map[string]model.Position)
	t := &trader{
		client:    client,
		settings:  settings,
		positions: positions,
		storage:   st,
		account:   id,
		running:   true,
		minSize:   minSize,
		config:    make(map[model.Coin]config),
		lock:      new(sync.RWMutex),
	}
	err = t.load()
	log.Err(err).Msg("could not load state")
	return t, nil
}

func (t *trader) buildState() State {
	return State{
		MinSize:   t.minSize,
		Running:   t.running,
		Positions: t.positions,
	}
}

func (t *trader) parseState(state State) {
	t.minSize = state.MinSize
	t.running = state.Running
	t.positions = state.Positions
}

func (t *trader) save() error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.storage.Store(stKey(), t.buildState())
}

func (t *trader) load() error {
	t.lock.Lock()
	defer t.lock.Unlock()
	state := State{
		Positions: make(map[string]model.Position),
	}
	err := t.storage.Load(stKey(), &state)
	if err != nil {
		return fmt.Errorf("could not load state: %w", err)
	}
	t.parseState(state)
	log.Info().Err(err).
		Str("account", t.account).
		Int("num", len(t.positions)).
		Bool("running", t.running).
		Int("min-size", t.minSize).
		Msg("loaded state")
	return nil
}

// Reset stops tracking positions for the given coins.
// This will leave the assets untouched in the exchange account.
func (t *trader) Reset(coins ...model.Coin) (map[string]model.Position, error) {
	positions := t.positions
	for _, coin := range coins {
		if string(coin) == "" {
			continue
		}
		newPositions := make(map[string]model.Position)
		for k, position := range positions {
			if position.Coin != coin {
				newPositions[k] = position
			}
		}
		positions = newPositions
	}
	t.positions = positions
	return t.positions, t.save()
}

// GetAll returns all open positions.
func (t *trader) GetAll(ctx context.Context) ([]string, map[string]model.Position, map[model.Coin]model.CurrentPrice) {
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
// Check checks the positions for the given strategy key and coin.
func (t *trader) Check(key model.Key, coin model.Coin) (model.Position, bool, []model.Position) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if p, ok := t.positions[key.Hash()]; ok {
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

// Submit the position state for the given order.
// This will treat the action as open or close, depending on the order reference.
func (t *trader) Submit(key model.Key, order *model.TrackedOrder, reference string) error {
	if reference != "" {
		if _, ok := t.positions[key.Hash()]; !ok {
			return fmt.Errorf("cannot find position to reference for key: %v", key)
		}
		delete(t.positions, key.Hash())
		return t.save()
	}
	// we need to be careful here and add the position ...
	position := model.OpenPosition(order, nil)
	if p, ok := t.positions[key.Hash()]; ok {
		if position.Coin != p.Coin {
			return fmt.Errorf("different coin found for key: %v [%s vs %s]", key, p.Coin, position.Coin)
		}
		if position.Type != p.Type {
			return fmt.Errorf("different type found for key: %v [%s vs %s]", key, p.Type.String(), position.Type.String())
		}
		position.Volume += p.Volume
		log.Warn().
			Str("account", t.account).
			Str("key", key.ToString()).
			Str("hash", key.Hash()).
			Float64("from", p.Volume).
			Float64("to", position.Volume).
			Msg("extending position")
	}
	t.positions[key.Hash()] = position
	return t.save()
}

func (t *trader) initConfig(c model.Coin) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if _, ok := t.config[c]; !ok {
		t.config[c] = newConfig(float64(t.minSize))
	}
}
