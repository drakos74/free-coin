package public

import (
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/client/kraken/model"
	"github.com/drakos74/free-coin/coinapi"
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
		Coin:   model.Coin(pair),
		Price:  trade.PriceFloat,
		Volume: trade.VolumeFloat,
		Time:   time.Unix(trade.Time, 0),
		Meta:   make(map[string]interface{}),
		Active: active,
		Type:   t,
	}
}
