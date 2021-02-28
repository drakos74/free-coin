package model

import (
	"strconv"
	"time"

	"github.com/drakos74/free-coin/client/local"

	krakenapi "github.com/beldur/kraken-go-api-client"
	coinmodel "github.com/drakos74/free-coin/internal/model"
)

// NewHistoryTrade creates a ew history trade from the given trade
func NewHistoryTrade(id string, trade krakenapi.TradeHistoryInfo) coinmodel.Trade {
	var refId string
	if len(trade.Trades) > 0 {
		refId = trade.Trades[0]
	}
	net, _ := strconv.ParseFloat(trade.Net, 64)
	return coinmodel.Trade{
		ID:     id,
		RefID:  refId,
		Net:    net,
		Coin:   Coin().Coin(trade.AssetPair),
		Price:  trade.Price,
		Volume: trade.Volume,
		Time:   time.Unix(int64(trade.Time), 0),
		Type:   Type().To(trade.Type),
	}
}

func NewTrackedPosition(id string, trade krakenapi.TradeHistoryInfo) local.TrackedPosition {
	return local.TrackedPosition{
		Open:     time.Time{},
		Close:    time.Time{},
		Position: coinmodel.Position{},
	}
}
