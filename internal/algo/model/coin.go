package model

import (
	"github.com/drakos74/free-coin/internal/api"
)

// Config contains coin related configuration.
var Coins = map[string]api.Coin{
	"BTC":   api.BTC,
	"ETH":   api.ETH,
	"EOS":   api.EOS,
	"LINK":  api.LINK,
	"WAVES": api.WAVES,
	"DOT":   api.DOT,
	"XRP":   api.XRP,
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
