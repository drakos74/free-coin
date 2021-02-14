package model

import (
	"github.com/drakos74/free-coin/internal/api"
)

// String returns the string representation for the Type.
func String(t api.Type) string {
	switch t {
	case api.Buy:
		return "buy"
	case api.Sell:
		return "sell"
	default:
		return ""
	}
}

// Sign returns the appropriate sign for the given Type.
func Sign(t api.Type) float64 {
	switch t {
	case api.Buy:
		return 1.0
	case api.Sell:
		return -1.0
	}
	return 0.0
}

// Inv inverts the Type from buy to sell and vice versa.
func Inv(t api.Type) api.Type {
	switch t {
	case api.Buy:
		return api.Sell
	case api.Sell:
		return api.Buy
	}
	return api.NoType
}
