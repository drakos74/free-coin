package storage

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/storage/file/json"
)

const (
	defaultDir = "file-storage"
	BookDir    = "book"
	HistoryDir = "history"
)

type Key struct {
	Since int64  `json:"since"`
	Pair  string `json:"pair"`
	Label string `json:"label"`
}

func (k Key) DateSince() time.Time {
	return time.Unix(k.Since/1000000000, 0)
}

func KeyFromPath(fileName string) Key {
	parts := strings.Split(fileName, "_")
	unixTime, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		panic(fmt.Sprintf("could not parse key from path: %s: %v", fileName, err))
	}
	return Key{
		Since: unixTime,
		Pair:  parts[0],
		Label: parts[1],
	}
}

func (k Key) path() string {
	return fmt.Sprintf("%s_%s_%v", k.Pair, k.Label, k.Since)
}

type Persistence interface {
	Store(k Key, value interface{}) error
	Load(k Key, value interface{}) error
}

type JsonBlobStorage struct {
	path  string
	table string
	shard string
}

func (s JsonBlobStorage) Store(k Key, value interface{}) error {
	return json.Save(filepath.Join(s.path, s.table, s.shard), k.path(), value)
}

func (s JsonBlobStorage) Load(k Key, value interface{}) error {
	return json.Load(filepath.Join(s.path, s.table, s.shard), k.path(), value)
}

// table has the same schema
// shard is a logical split
func NewJsonBlob(table, shard string) *JsonBlobStorage {
	return &JsonBlobStorage{table: table, shard: shard, path: defaultDir}
}

type DummyStorage struct {
}

func (d DummyStorage) Store(k Key, value interface{}) error {
	return nil
}

func (d DummyStorage) Load(k Key, value interface{}) error {
	return fmt.Errorf("not found: %v", k)
}

func Dummy() *DummyStorage {
	return &DummyStorage{}
}
