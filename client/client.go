package client

import (
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
)

type Source interface {
	Trades(process <-chan api.Signal) (coinmodel.TradeSource, error)
}

// Factory defines the factory interface for a client.
type Factory func(since int64) (api.Client, error)

// TODO : create multi-client
