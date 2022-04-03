package trader

import (
	"fmt"
	"path"
	"sync"

	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const storagePath = "trader"

type trader struct {
	positions map[model.Key]model.Position
	storage   storage.Persistence
	account   string
	running   bool
	minSize   int
	config    []*model.TrackingConfig
	lock      *sync.RWMutex
}

func newTrader(id string, shard storage.Shard, config []*model.TrackingConfig) (*trader, error) {
	st, err := shard(path.Join(storagePath, id))
	if err != nil {
		return nil, fmt.Errorf("could not init storage: %w", err)
	}
	positions := make(map[model.Key]model.Position)
	t := &trader{
		positions: positions,
		storage:   st,
		account:   id,
		running:   true,
		minSize:   minSize,
		config:    config,
		lock:      new(sync.RWMutex),
	}
	err = t.load()
	return t, err
}

func (t *trader) buildState() State {
	positions := make(map[string]model.Position)
	for k, p := range t.positions {
		positions[k.ToString()] = p
	}
	return State{
		MinSize:   t.minSize,
		Running:   t.running,
		Positions: positions,
	}
}

func (t *trader) parseState(state State) {
	t.minSize = state.MinSize
	t.running = state.Running
	positions := make(map[model.Key]model.Position)
	for k, p := range state.Positions {
		positions[FromString(k)] = p
	}
	t.positions = positions
}

func (t *trader) save() error {
	return t.storage.Store(stKey(t.account), t.buildState())
}

func (t *trader) load() error {
	state := State{
		Positions: make(map[string]model.Position),
	}
	err := t.storage.Load(stKey(t.account), &state)
	t.parseState(state)
	log.Info().Err(err).
		Str("account", t.account).
		Int("num", len(t.positions)).
		Bool("running", t.running).
		Int("min-size", t.minSize).
		Msg("loaded state")
	return nil
}

func (t *trader) update(trade *model.TradeSignal) map[model.Key]model.Position {
	positions := make(map[model.Key]model.Position)
	newPositions := make(map[model.Key]model.Position)
	ip := 0
	pp := t.positions
	for k, p := range pp {
		if k.Match(trade.Coin) {
			p = p.Update(trade.Tick)
			positions[k] = p
			ip++
		}
		newPositions[k] = p
	}
	// TODO : think if its worth to have this at every call
	t.positions = newPositions
	//err := t.save()
	//if err != nil {
	//	log.Error().Err(err).Int("num", ip).Str("coin", string(trade.Coin)).Msg("could not update position")
	//}
	return positions
}

func (t *trader) reset(coins ...model.Coin) (map[model.Key]model.Position, error) {
	positions := t.positions
	if len(coins) == 1 && coins[0] == model.AllCoins {
		positions = make(map[model.Key]model.Position)
	} else {
		for _, coin := range coins {
			if coin == model.NoCoin {
				continue
			}
			newPositions := make(map[model.Key]model.Position)
			for k, position := range positions {
				if position.Coin != coin {
					newPositions[k] = position
				}
			}
			positions = newPositions
		}
	}
	t.positions = positions
	return t.positions, t.save()
}

func (t *trader) getAll(coins ...model.Coin) ([]model.Key, map[model.Key]model.Position /*, map[model.Coin]model.CurrentPrice*/) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	positions := make(map[model.Key]model.Position)
	keys := make([]model.Key, 0)
	for _, c := range coins {
		for k, p := range t.positions {
			if c == model.AllCoins || k.Match(c) {
				positions[k] = p
				keys = append(keys, k)
			}
		}
	}
	return keys, positions //, prices
}

// check checks if we have a position for the given key
// if not, but there are positions for the same coin, it will return them in the slice
func (t *trader) check(key model.Key) (model.Position, bool, map[model.Key]model.Position) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if p, ok := t.positions[key]; ok {
		return p, true, map[model.Key]model.Position{}
	}
	// TODO : remove this at some point.
	// We want ... for now to ... basically avoid closing with the same coin but different key
	positions := make(map[model.Key]model.Position)
	for k, p := range t.positions {
		if p.Coin == key.Coin {
			positions[k] = p
		}
	}
	return model.Position{}, false, positions
}

func (t *trader) close(key model.Key) error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if _, ok := t.positions[key]; !ok {
		return fmt.Errorf("cannot find position to close for key: %s", key)
	}
	delete(t.positions, key)
	return t.save()
}

func (t *trader) add(key model.Key, order *model.TrackedOrder, live bool) error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	// we need to be careful here and add the position ...
	position := model.OpenPosition(order, t.config)
	position.Live = live
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
			Str("key", fmt.Sprintf("%+v", key)).
			Float64("from", p.Volume).
			Float64("to", position.Volume).
			Msg("extending position")
	}
	t.positions[key] = position
	return t.save()
}

func stKey(account string) storage.Key {
	return storage.Key{
		Pair:  "all",
		Hash:  0,
		Label: account,
	}
}
