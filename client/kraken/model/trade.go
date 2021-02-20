package model

import "github.com/drakos74/free-coin/internal/model"

// TradeBatch defines a generic grouping of trades
type TradeBatch struct {
	Trades []model.Trade
	Index  int64
}
