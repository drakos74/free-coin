package account

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
)

// Secret defines a security pair of a key and secret
type Secret struct {
	Key    string
	Secret string
}

// Token represents a tokenized secret combination of a string and an ID.
type Token struct {
	Token string
	ID    int
}

// Mapping is the config mapping to mao users and keywords across implementations.
type Mapping struct {
	m map[string]string
}

// NewMapping creates a new mapping object
func NewMapping() *Mapping {
	return &Mapping{m: make(map[string]string)}
}

func (m *Mapping) Add(name string, alias ...string) error {
	err := m.add(name, name)
	if err != nil {
		return fmt.Errorf("could not add name: %w", err)
	}
	for _, a := range alias {
		err = m.add(a, name)
		if err != nil {
			return fmt.Errorf("could not add alias for ''%s: %w", name, err)
		}
	}
	return nil
}

func (m *Mapping) add(key, value string) error {
	if _, ok := m.m[key]; ok {
		return fmt.Errorf("key '%s' already exists", key)
	}
	m.m[key] = value
	return nil
}

// Details define the account details.
type Details struct {
	Name       Name
	Alias      []string
	Multiplier float64
	Exchange   ExchangeDetails
	User       UserDetails
}

// ExchangeDetails are the exchange specific details.
type ExchangeDetails struct {
	Name   api.ExchangeName
	Margin bool
}

// UserDetails are the user communication details.
type UserDetails struct {
	Index  api.Index
	ChatID int64
	Alias  string
}
