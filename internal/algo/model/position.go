package model

import "github.com/drakos74/free-coin/coinapi"

func OpenPosition(coin coinapi.Coin, t coinapi.Type) coinapi.Position {
	return coinapi.Position{
		Coin: coin,
		Type: t,
	}
}
