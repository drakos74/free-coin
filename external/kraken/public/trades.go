package public

import (
	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/coinapi"
	"github.com/drakos74/free-coin/internal/time"
	"github.com/drakos74/free-coin/kraken/api"
)

type TradeInfo chan<- krakenapi.TradeInfo

func NewTrade(pair string, active bool, trade krakenapi.TradeInfo) coinapi.Trade {
	var t coinapi.Type
	if trade.Buy {
		t = coinapi.Buy
	} else if trade.Sell {
		t = coinapi.Sell
	}
	return coinapi.Trade{
		Coin:   api.Coin(pair),
		Price:  trade.PriceFloat,
		Volume: trade.VolumeFloat,
		Time:   time.New(trade.Time),
		Meta:   make(map[string]interface{}),
		Active: active,
		Type:   t,
	}
}
