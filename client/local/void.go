package local

import (
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

type VoidClient struct {
}

func Void() api.Client {
	return &VoidClient{}
}

func (v VoidClient) Trades(process <-chan api.Action, query api.Query) (model.TradeSource, error) {
	// dont send anything ... actually even better , close the channel
	trades := make(chan *model.Trade)
	go func() {
		<-time.NewTicker(1 * time.Millisecond).C
		close(trades)
	}()
	return trades, nil
}
