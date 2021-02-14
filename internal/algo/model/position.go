package model

import (
	"github.com/drakos74/free-coin/internal/api"
)

func OpenPosition(coin api.Coin, t api.Type) api.Position {
	return api.Position{
		Coin: coin,
		Type: t,
	}
}
