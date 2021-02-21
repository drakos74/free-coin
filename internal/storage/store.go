package storage

import (
	"errors"
	"fmt"
)

const (
	BookDir    = "book"
	HistoryDir = "history"
)

var (
	NotFoundErr      = errors.New("not found")
	CouldNotLoadErr  = errors.New("could not load")
	UnrecoverableErr = errors.New("unrecoverable error")
)

type Key struct {
	Hash  int64  `json:"hash"`
	Pair  string `json:"pair"`
	Label string `json:"label"`
}

func (k Key) Path() string {
	return fmt.Sprintf("%s_%v_%s", k.Pair, k.Hash, k.Label)
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

func Void() *VoidStorage {
	return &VoidStorage{}
}
