package kraken

import (
	"fmt"

	"github.com/drakos74/free-coin/coinapi"
	"github.com/drakos74/free-coin/kraken/api"
)

var (
	leverage = map[string]coinapi.Leverage{
		api.Pair(coinapi.BTC):  coinapi.L_5,
		api.Pair(coinapi.ETH):  coinapi.L_5,
		api.Pair(coinapi.DOT):  coinapi.L_3,
		api.Pair(coinapi.LINK): coinapi.L_3,
		api.Pair(coinapi.XRP):  coinapi.L_5,
	}
)

// Leverage returns the pre-defined max leverage for the kraken exchange
func Leverage(pair string) coinapi.Leverage {
	if l, ok := leverage[pair]; ok {
		return l
	}
	panic(fmt.Sprintf("no leverage defined for pair: %s", pair))
}
