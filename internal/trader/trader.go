package trader

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

type State struct {
	MinSize   int                       `json:"min_size"`
	Running   bool                      `json:"running"`
	Positions map[string]model.Position `json:"positions"`
}

type config struct {
	minSize int
	running bool
}

type Trader struct {
	id        string
	client    api.Exchange
	positions map[string]model.Position
	storage   storage.Persistence
	lock      *sync.RWMutex
	config
}

func NewTrader(id string, shard storage.Shard) (*Trader, error) {
	st, err := shard(id)
	if err != nil {
		return nil, fmt.Errorf("could not init storage: %w", err)
	}
	positions := make(map[string]model.Position)
	t := &Trader{
		//client:    client,
		positions: positions,
		storage:   st,
		lock:      new(sync.RWMutex),
	}
	err = t.load()
	return t, err
}

func (t *Trader) buildState() State {
	return State{
		MinSize:   t.minSize,
		Running:   t.running,
		Positions: t.positions,
	}
}

func (t *Trader) parseState(state State) {
	t.minSize = state.MinSize
	t.running = state.Running
	t.positions = state.Positions
}

func (t *Trader) save() error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.storage.Store(t.stKey(), t.buildState())
}

func (t *Trader) load() error {
	t.lock.Lock()
	defer t.lock.Unlock()
	state := State{
		Positions: make(map[string]model.Position),
	}
	err := t.storage.Load(t.stKey(), &state)
	if err != nil {
		return fmt.Errorf("could not load state: %w", err)
	}
	t.parseState(state)
	log.Info().Err(err).
		Int("num", len(t.positions)).
		Bool("running", t.running).
		Int("min-size", t.minSize).
		Msg("loaded state")
	return nil
}

func (t *Trader) reset(coins ...model.Coin) (map[string]model.Position, error) {
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

func (t *Trader) getAll(ctx context.Context) ([]string, map[string]model.Position, map[model.Coin]model.CurrentPrice) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	//prices, err := t.client.CurrentPrice(ctx)
	//if err != nil {
	//	log.Error().Err(err).
	//		Msg("could not get current prices")
	prices := make(map[model.Coin]model.CurrentPrice)
	//}

	positions := make(map[string]model.Position)
	keys := make([]string, 0)
	for k, p := range t.positions {
		// check the current price
		if cp, ok := prices[p.Coin]; ok {
			p.CurrentPrice = cp.Price
			p.Value(model.NewPrice(cp.Price, time.Now()))
		}
		positions[k] = p
		keys = append(keys, k)
	}
	return keys, positions, prices
}

// TODO : make this for now ... to have clear pairs
func (t *Trader) check(key string, coin model.Coin) (model.Position, bool, []model.Position) {
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

func (t *Trader) add(key string, order *model.TrackedOrder, close string) (profit float64, err error) {
	if close != "" {
		if _, ok := t.positions[key]; !ok {
			return profit, fmt.Errorf("cannot find position to close for key: %s", key)
		}
		diff := 0.0
		oldPos := t.positions[key]
		// check the diff
		switch oldPos.Type {
		case model.Buy:
			diff = order.Price - oldPos.OpenPrice
		case model.Sell:
			diff = oldPos.OpenPrice - order.Price
		}
		delete(t.positions, key)
		return 100 * diff / order.Price, t.save()
	}
	// we need to be careful here and add the position ...
	position := model.OpenPosition(order)
	if p, ok := t.positions[key]; ok {
		if position.Coin != p.Coin {
			return profit, fmt.Errorf("different coin found for key: %s [%s vs %s]", key, p.Coin, position.Coin)
		}
		if position.Type != p.Type {
			return profit, fmt.Errorf("different type found for key: %s [%s vs %s]", key, p.Type.String(), position.Type.String())
		}
		position.Volume += p.Volume
		log.Warn().
			Str("key", key).
			Float64("from", p.Volume).
			Float64("to", position.Volume).
			Msg("extending position")
	}
	t.positions[key] = position
	return profit, t.save()
}

func (t *Trader) stKey() storage.Key {
	return storage.Key{
		Pair:  "all",
		Hash:  0,
		Label: t.id,
	}
}
