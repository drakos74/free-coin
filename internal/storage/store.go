package storage

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
)

const (
	BookDir     = "book"
	HistoryDir  = "history"
	RegistryDir = "registry"

	BackTestRegistryPath = "backtest-events"
	EventsPath           = "events"
	SignalsPath          = "signals"

	BackTestInternalPath = "backtest-internal"
	InternalPath         = "internal"
)

var (
	// DefaultDir is the default storage directory
	// TODO : leaving this for now to be able to adjust for the tests
	DefaultDir = "file-storage"
)

// Shard creates a new storage implementation for the given shard.
type Shard func(shard string) (Persistence, error)

// EventRegistry creates a registry for the given path
type EventRegistry func(path string) (Registry, error)

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

// Registry is a storage pattern like a logger or event registry.
// It receives events one by one, but loads all of them at once.
type Registry interface {
	Add(key K, value interface{}) error
	GetAll(key K, value interface{}) error
	GetFor(key K, value interface{}, filter func(s string) bool) error
	Check(key K) (map[string]RegistryPath, error)
	Root() string
}

// RegistryPath defines a registry path with including files
type RegistryPath struct {
	Name  string
	Files []string
}

// Persistence is a batch storage that offers the functionality to store and load large objects at once.
type Persistence interface {
	Store(k Key, value interface{}) error
	Load(k Key, value interface{}) error
}

func Store(persistence Persistence, key Key, value interface{}) {
	err := persistence.Store(key, value)
	if err != nil {
		log.Error().
			Str("key", fmt.Sprintf("%+v", key)).
			Err(err).
			Msg("could not store")
	}
}

func Add(registry Registry, key K, value interface{}) {
	err := registry.Add(key, value)
	if err != nil {
		log.Error().
			Str("key", fmt.Sprintf("%+v", key)).
			Err(err).
			Msg("could not add")
	}
}

func ToFile(path string, name string, ext string, content []fmt.Stringer) (string, error) {
	fn, err := MakePath(path, fmt.Sprintf("%s.%s", name, ext))
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer file.Close()

	if err != nil {
		return "", fmt.Errorf("could not open file: %w", err)
	}

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// take only the last n samples
	for _, line := range content {
		_, _ = writer.WriteString(line.String() + "\n")
	}
	return fn, nil
}

func MakePath(parentDir string, fileName string) (string, error) {
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		err := os.MkdirAll(parentDir, 0700) // Create your file
		if err != nil {
			return "", err
		}
	}
	fileName = fmt.Sprintf("%s/%s", parentDir, fileName)
	//file, _ := os.Create(fileName)
	//defer file.Close()
	return fileName, nil
}
