package model

import "github.com/drakos74/free-coin/coinapi"

// Config contains coin related configuration.
var Coins = map[string]coinapi.Coin{
	"BTC":   coinapi.BTC,
	"ETH":   coinapi.ETH,
	"EOS":   coinapi.EOS,
	"LINK":  coinapi.LINK,
	"WAVES": coinapi.WAVES,
	"DOT":   coinapi.DOT,
	"XRP":   coinapi.XRP,
}

func KnownCoins() []string {
	cc := make([]string, len(Coins))
	i := 0
	for c := range Coins {
		cc[i] = c
		i++
	}
	return cc
}
