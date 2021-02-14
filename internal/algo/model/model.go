package model

import (
	"github.com/drakos74/free-coin/coinapi"
)

// String returns the string representation for the Type.
func String(t coinapi.Type) string {
	switch t {
	case coinapi.Buy:
		return "buy"
	case coinapi.Sell:
		return "sell"
	default:
		return ""
	}
}

// Sign returns the appropriate sign for the given Type.
func Sign(t coinapi.Type) float64 {
	switch t {
	case coinapi.Buy:
		return 1.0
	case coinapi.Sell:
		return -1.0
	}
	return 0.0
}

// Inv inverts the Type from buy to sell and vice versa.
func Inv(t coinapi.Type) coinapi.Type {
	switch t {
	case coinapi.Buy:
		return coinapi.Sell
	case coinapi.Sell:
		return coinapi.Buy
	}
	return coinapi.NoType
}
