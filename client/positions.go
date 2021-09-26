package client

import (
	"fmt"
	"path"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/model"
)

// Positions defines a position storage interface.
type Positions interface {
	Check(key model.Key, coin model.Coin) (model.Position, bool, []model.Position)
	Close(key model.Key) error
	Submit(key model.Key, position model.Position) error
	GetAll() map[string]model.Position
}

// CachedPositionStorage is a local storage implementation for the positions.
type CachedPositionStorage struct {
	lock      *sync.RWMutex
	positions map[string]model.Position
	storage   storage.Persistence
}

// NewLocalPositionStorage initialises a local position storage
func NewLocalPositionStorage(userID string, shard storage.Shard) (*CachedPositionStorage, error) {
	st, err := shard(path.Join("exchange", userID))
	if err != nil {
		return nil, fmt.Errorf("could not init storage for positions: %w", err)
	}
	localStorage := &CachedPositionStorage{
		lock:    new(sync.RWMutex),
		storage: st,
	}
	positions, err := localStorage.load()
	if err != nil {
		// thats fine if it s new, so we init with empty positions ...
		// TODO : Or should we check the exchange itself ? Maybe in the upper layer ...
		log.Info().Str("user-id", userID).Err(err).Msg("could not load positions")
	}
	localStorage.positions = positions
	return localStorage, nil
}

// Check returns the position for the given key
// ... but also all other positions for the given coin.
func (st *CachedPositionStorage) Check(key model.Key, coin model.Coin) (model.Position, bool, []model.Position) {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if p, ok := st.positions[key.Hash()]; ok {
		return p, true, []model.Position{}
	}
	// TODO : remove this at some point.
	// We want ... for now to ... basically avoid closing with the same coin but different key
	positions := make([]model.Position, 0)
	for _, p := range st.positions {
		if p.Coin == coin {
			positions = append(positions, p)
		}
	}
	return model.Position{}, false, positions
}

// Close removes a position from the current open positions storage.
func (st *CachedPositionStorage) Close(key model.Key) error {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if _, ok := st.positions[key.Hash()]; !ok {
		return fmt.Errorf("cannot find position to reference for key: %v", key)
	}
	delete(st.positions, key.Hash())
	return st.save()
}

// Submit adds a position to the storage.
func (st *CachedPositionStorage) Submit(key model.Key, position model.Position) error {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if p, ok := st.positions[key.Hash()]; ok {
		if position.Coin != p.Coin {
			return fmt.Errorf("different coin found for key: %v [%s vs %s]", key, p.Coin, position.Coin)
		}
		if position.Type != p.Type {
			return fmt.Errorf("different type found for key: %v [%s vs %s]", key, p.Type.String(), position.Type.String())
		}
		position.Volume += p.Volume
	}
	st.positions[key.Hash()] = position
	return st.save()
}

func (st *CachedPositionStorage) GetAll() map[string]model.Position {
	st.lock.Lock()
	defer st.lock.Unlock()
	positions := make(map[string]model.Position, len(st.positions))
	for k, pos := range st.positions {
		positions[k] = pos
	}
	return positions
}

func (st *CachedPositionStorage) save() error {
	return st.storage.Store(stKey(), st.positions)
}

func (st *CachedPositionStorage) load() (map[string]model.Position, error) {
	positions := make(map[string]model.Position)
	err := st.storage.Load(stKey(), &positions)
	return positions, err
}

// stKey creates the storage key for the trading processor.
func stKey() storage.Key {
	return storage.Key{
		Pair:  "all",
		Hash:  1,
		Label: "positions",
	}
}
