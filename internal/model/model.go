package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const delimiter = "-"

// Key characterises distinct processor attributes
type Key struct {
	Coin     Coin          `json:"coin"`
	Duration time.Duration `json:"duration"`
	Index    int           `json:"index"`
	Strategy string        `json:"strategy"`
}

// ToString creates a string representation of the key.
func (k Key) ToString() string {
	return fmt.Sprintf("%s%s%d%s%s%s%d",
		k.Coin, delimiter,
		int(k.Duration.Minutes()), delimiter,
		k.Strategy, delimiter,
		k.Index)
}

// ToString creates a string representation of the key.
func NewKeyFromString(cid string) (Key, error) {
	parts := strings.Split(cid, delimiter)
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
	i, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return Key{}, fmt.Errorf("could not de-correlate '%s': %w", cid, err)
	}
	index := int(i)
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
