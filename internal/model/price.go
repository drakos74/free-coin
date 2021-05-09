package model

import "time"

// CurrentPrice defines a price related to a coin.
type CurrentPrice struct {
	Coin  Coin
	Price float64
}

// Price defines a price value in time.
type Price struct {
	Value float64
	Time  time.Time
}

// NewPrice creates a new reference to a price.
func NewPrice(price float64, time time.Time) *Price {
	return &Price{
		Value: price,
		Time:  time,
	}
}
