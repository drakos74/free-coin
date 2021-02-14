package model

import (
	"github.com/drakos74/free-coin/internal/api"
)

// TradeBatch defines a generic grouping of trades
type TradeBatch struct {
	Trades []api.Trade
	Index  int64
}
