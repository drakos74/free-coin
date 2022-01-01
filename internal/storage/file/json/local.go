package json

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/drakos74/free-coin/internal/storage"
)

func LocalShard() func(shard string) (storage.Persistence, error) {
	return func(shard string) (storage.Persistence, error) {
		return newLocalStorage(), nil
	}
}

type LocalStorage struct {
	files map[storage.Key]string
	mutex *sync.RWMutex
}

func newLocalStorage() *LocalStorage {
	return &LocalStorage{
		files: make(map[storage.Key]string),
		mutex: new(sync.RWMutex),
	}
}

func (l LocalStorage) Store(k storage.Key, value interface{}) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	bb, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("could not marshal value: %w", err)
	}

	l.files[k] = string(bb)
	return nil
}

func (l LocalStorage) Load(k storage.Key, value interface{}) error {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	if v, ok := l.files[k]; ok {
		err := json.Unmarshal([]byte(v), value)
		if err != nil {
			return fmt.Errorf("could not unmarshal value: %w", err)
		}
		return nil
	}
	return fmt.Errorf("file not found: %+v", k)
}
