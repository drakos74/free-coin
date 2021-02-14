package api

import "github.com/drakos74/free-coin/coinapi"

// TradeBatch defines a generic grouping of trades
type TradeBatch struct {
	Trades []coinapi.Trade
	Index  int64
}
