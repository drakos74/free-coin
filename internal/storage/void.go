package storage

import "fmt"

// VoidStorage is a noop storage
type VoidStorage struct {
}

func (d VoidStorage) Store(k Key, value interface{}) error {
	return nil
}

func (d VoidStorage) Load(k Key, value interface{}) error {
	return fmt.Errorf("not found '%v': %w", k, NotFoundErr)
}

// NewVoidStorage creates a new noop storage
func NewVoidStorage() *VoidStorage {
	return &VoidStorage{}
}

// VoidShard creates a new noop shard
func VoidShard(table string) Shard {
	return func(shard string) (Persistence, error) {
		return NewVoidStorage(), nil
	}
}

// VoidRegistry is a dummy event logger which ignores all calls
type VoidRegistry struct {
}

// NewVoidRegistry creates a new noop registry
func NewVoidRegistry() *VoidRegistry {
	return &VoidRegistry{}
}

func (v VoidRegistry) Root() string {
	return ""
}

func (v VoidRegistry) Add(key K, value interface{}) error {
	return nil
}

func (v VoidRegistry) GetAll(key K, value interface{}) error {
	return nil
}

func (v VoidRegistry) GetFor(key K, value interface{}, filter func(s string) bool) error {
	return nil
}

func (v VoidRegistry) Check(key K) ([]string, error) {
	panic("implement me")
}
