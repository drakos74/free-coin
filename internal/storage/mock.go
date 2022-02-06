package storage

func MockShard() Shard {
	return func(shard string) (Persistence, error) {
		return NewMockStorage(), nil
	}
}

type MockStorage struct {
	Elements map[Key]interface{}
}

func NewMockStorage() *MockStorage {
	return &MockStorage{Elements: make(map[Key]interface{})}
}

func (m *MockStorage) Store(k Key, value interface{}) error {
	m.Elements[k] = value
	return nil
}

func (m *MockStorage) Load(k Key, value interface{}) error {
	return nil
}

func MockEventRegistry() EventRegistry {
	return func(path string) (Registry, error) {
		return NewMockRegistry(), nil
	}
}

type MockRegistry struct {
	Events map[K][]interface{}
}

func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		Events: make(map[K][]interface{}),
	}
}

func (m *MockRegistry) Add(key K, value interface{}) error {
	if _, ok := m.Events[key]; !ok {
		m.Events[key] = make([]interface{}, 0)
	}
	m.Events[key] = append(m.Events[key], value)
	return nil
}

func (m *MockRegistry) GetAll(key K, value interface{}) error {
	return nil
}

func (m *MockRegistry) GetFor(key K, value interface{}, filter func(s string) bool) error {
	return nil
}

func (m *MockRegistry) Root() string {
	return ""
}

func (m *MockRegistry) Check(key K) (map[string]RegistryPath, error) {
	panic("implement me")
}
