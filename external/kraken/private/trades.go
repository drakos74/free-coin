package private

import (
	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/coinapi"
	"github.com/drakos74/free-coin/kraken/api"
)

func NewHistoryTrade(id string, trade krakenapi.TradeHistoryInfo) coinapi.Trade {
	return coinapi.Trade{
		ID:     id,
		Coin:   api.Coin(trade.AssetPair),
		Price:  trade.Price,
		Volume: trade.Volume,
		Time:   cointime.New(int64(trade.Time)),
		Type:   api.Type(trade.Type),
		// TODO : add the positionInfo
		Meta: nil,
	}
}
