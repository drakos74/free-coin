package storage

import (
	"errors"
	"fmt"
)

const (
	BookDir      = "book"
	HistoryDir   = "history"
	RegistryDir  = "registry"
	RegistryPath = "events"
)

var (
	// TODO : leaving this for now to be able to adjust for the tests
	DefaultDir = "file-storage"
)

// Shard creates a new storage implementation for the given shard.
type Shard func(shard string) (Persistence, error)

var (
	NotFoundErr      = errors.New("not found")
	CouldNotLoadErr  = errors.New("could not load")
	UnrecoverableErr = errors.New("unrecoverable error")
)

// Key is the storage key for a general implementation
type Key struct {
	Hash  int64  `json:"hash"`
	Pair  string `json:"pair"`
	Label string `json:"label"`
}

// K is a simplified key for storage
type K struct {
	Pair  string `json:"pair"`
	Label string `json:"label"`
}

func (k Key) Path() string {
	return fmt.Sprintf("%s_%v_%s", k.Pair, k.Hash, k.Label)
}

// Hashed is a pre-hashed storage for storing items one by one.
type Registry interface {
	Put(key K, value interface{}) error
	Get(key K, value interface{}) error
}

type Persistence interface {
	Store(k Key, value interface{}) error
	Load(k Key, value interface{}) error
}

type VoidStorage struct {
}

func (d VoidStorage) Store(k Key, value interface{}) error {
	return nil
}

func (d VoidStorage) Load(k Key, value interface{}) error {
	return fmt.Errorf("not found '%v': %w", k, NotFoundErr)
}

func NewVoidStorage() *VoidStorage {
	return &VoidStorage{}
}

// NewVoidStorage is a dummy event logger which ignores all calls
type VoidRegistry struct {
}

func NewVoidRegistry() *VoidRegistry {
	return &VoidRegistry{}
}

func (v VoidRegistry) Put(key K, value interface{}) error {
	return nil
}

func (v VoidRegistry) Get(key K, value interface{}) error {
	return nil
}
