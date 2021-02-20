package model

import (
	"strconv"

	"github.com/rs/zerolog/log"
)

// Formatter is a number formatter interface to format floats to readable strings.
type Formatter interface {
	Format(c Coin, f float64) string
}

// CoinFormatter is a formatter based on the coin
type CoinFormatter struct {
	precision map[Coin]int
}

// Formats formats the given value for the provided coin.
func (p CoinFormatter) Format(c Coin, f float64) string {
	precision := p.precision[c]
	if _, ok := p.precision[c]; !ok {
		log.Warn().
			Float64("value", f).
			Str("coin", string(c)).
			Msg("unknown precision for coin")
		precision = 0
	}
	price := strconv.FormatFloat(f, 'f', precision, 64)
	return price
}

// Price is a formatter for the price of the coin.
var Price = CoinFormatter{precision: map[Coin]int{
	BTC:  0,
	ETH:  0,
	DOT:  2,
	LINK: 2,
	XRP:  2,
}}

// Volume is a formatter for the volume of the coin.
var Volume = CoinFormatter{precision: map[Coin]int{
	BTC:  2,
	ETH:  2,
	DOT:  0,
	LINK: 0,
	XRP:  0,
}}