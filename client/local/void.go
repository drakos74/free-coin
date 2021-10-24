package local

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/client"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

// VoidFactory is a factory returning a void client.
func VoidFactory() client.Factory {
	return func(since int64) (api.Client, error) {
		return Void(), nil
	}
}

type VoidClient struct {
}

func Void() api.Client {
	return &VoidClient{}
}

func (v VoidClient) Trades(process <-chan api.Signal) (model.TradeSource, error) {
	// dont send anything ... actually even better , close the channel
	trades := make(chan *model.Trade)
	go func() {
		<-time.NewTicker(1 * time.Millisecond).C
		log.Info().Str("processor", "local-void").Msg("closing processor")
		close(trades)
	}()
	return trades, nil
}

func (v VoidClient) CurrentPrice(ctx context.Context) (map[model.Coin]model.CurrentPrice, error) {
	return make(map[model.Coin]model.CurrentPrice), nil
}
