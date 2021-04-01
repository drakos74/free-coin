package binance

import (
	"fmt"

	"github.com/adshao/go-binance/v2"
	"github.com/drakos74/free-coin/client/binance/model"
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// Client is the trades client implementation for binance.
type Client struct {
	converter model.Converter
}

// New creates a new trades client for binance.
func NewClient() *Client {
	return &Client{
		converter: model.NewConverter(),
	}
}

// Trades returns a channel with the trades from binance.
func (c *Client) Trades(process <-chan api.Action, query api.Query) (coinmodel.TradeSource, error) {
	tradeSource := make(chan *coinmodel.Trade)

	wsKlineHandler := func(event *binance.WsKlineEvent) {
		trade, err := model.FromKLine(event)
		if err != nil {
			log.Error().
				Err(err).
				Str("kline", fmt.Sprintf("%+v", event)).
				Msg("binance kline")
		}
		tradeSource <- trade
	}
	errHandler := func(err error) {
		log.Error().Err(err).Msg("error for socket connection to binance")
	}
	log.Info().Str("query", fmt.Sprintf("%+v", query)).Msg("starting listener")
	doneC, _, err := binance.WsKlineServe(c.converter.Coin.Pair(query.Coin), "1m", wsKlineHandler, errHandler)
	if err != nil {
		log.Error().Err(err).Msg("could not init socket connection")
		return tradeSource, fmt.Errorf("could not init socket: %w", err)
	}

	go func() {
		<-doneC
		log.Info().Msg("done processing trades from binance")
		close(tradeSource)
	}()

	return tradeSource, nil
}
