package local

import (
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

type VoidClient struct {
}

func Void() api.Client {
	return &VoidClient{}
}

func (v VoidClient) Trades(stop <-chan struct{}, coin model.Coin, stopExecution api.Condition) (model.TradeSource, error) {
	// dont send anything ... actually even better , close the channel
	trades := make(chan *model.Trade)
	defer close(trades)
	return trades, nil
}
