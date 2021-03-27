package model

import (
	"fmt"
	"strconv"

	"github.com/adshao/go-binance/v2"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
)

func FromKLine(event *binance.WsKlineEvent) (*coinmodel.Trade, error) {
	openPrice, err := strconv.ParseFloat(event.Kline.Open, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse openPrice price: %w", err)
	}
	closePrice, err := strconv.ParseFloat(event.Kline.Close, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse closePrice price: %w", err)
	}
	volume, err := strconv.ParseFloat(event.Kline.Volume, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse volume: %w", err)
	}
	trade := coinmodel.Trade{
		SourceID: "binance",
		ID:       strconv.FormatInt(event.Kline.LastTradeID, 10),
		Coin:     coinmodel.Coin(event.Symbol),
		Price:    (openPrice + closePrice) / float64(2),
		Volume:   volume,
		Time:     cointime.FromMilli(event.Kline.EndTime),
		Type:     coinmodel.SignedType(closePrice - openPrice),
		Active:   true,
		Live:     true,
		Signals:  make([]coinmodel.Signal, 0),
	}
	return &trade, nil
}
