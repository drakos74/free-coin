package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const Delimiter = "_"

// Key characterises distinct processor attributes
type Key struct {
	Coin     Coin          `json:"coin"`
	Duration time.Duration `json:"duration"`
	Index    int64         `json:"index"`
	Strategy string        `json:"strategy"`
}

func TempKey(c Coin, d time.Duration) Key {
	return Key{
		Coin:     c,
		Duration: d,
	}
}

// Match evaluates if the given trade matches the current key.
func (k Key) Match(c Coin) bool {
	return k.Coin == c
}

// Hash creates a string representation of the key.
func (k Key) Hash() string {
	return fmt.Sprintf("%s%s%d%s%d%s%s",
		k.Coin, Delimiter,
		int(k.Duration.Minutes()), Delimiter,
		k.Index, Delimiter,
		k.Strategy)
}

// ToString creates a string representation of the key.
func (k Key) ToString() string {
	return fmt.Sprintf("%s%s%d%s%s%s%d",
		k.Coin, Delimiter,
		int(k.Duration.Minutes()), Delimiter,
		k.Strategy, Delimiter,
		k.Index)
}

// NewKeyFromString creates a string representation of the key.
func NewKeyFromString(cid string) (Key, error) {
	parts := strings.Split(cid, Delimiter)
	if len(parts) != 4 {
		return Key{}, fmt.Errorf("could not de-correlate '%s'", cid)
	}
	coin := Coin(parts[0])
	strategy := parts[2]
	d, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Key{}, fmt.Errorf("could not de-correlate '%s': %w", cid, err)
	}
	duration := time.Duration(d) * time.Minute
	index, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return Key{}, fmt.Errorf("could not de-correlate '%s': %w", cid, err)
	}
	return Key{
		Coin:     coin,
		Duration: duration,
		Index:    index,
		Strategy: strategy,
	}, nil
}

// NewKey creates a new processor key
func NewKey(c Coin, d time.Duration, strategy string) Key {
	return Key{
		Coin:     c,
		Duration: d,
		Strategy: strategy,
	}
}
