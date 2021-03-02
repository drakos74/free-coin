package model

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/client/local"

	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
)

func ToKey(coin coinmodel.Coin, timeRange cointime.Range) []storage.Key {

	keys := make([]storage.Key, 0)

	hash := cointime.NewHash(8 * time.Hour)
	fromkey := hash.Do(timeRange.From)
	toKey := hash.Do(timeRange.To)

	for h := fromkey; h <= toKey; h++ {
		keys = append(keys, storage.Key{
			Hash: h,
			Pair: string(coin),
		})
	}
	return keys
}

func TradeData(field string, trade coinmodel.Trade) []float64 {
	t := Time(trade.Time)
	switch field {
	case "price":
		return []float64{trade.Price, t}
	case "volume":
		return []float64{trade.Type.Sign() * trade.Volume, t}
	default:
		log.Warn().Str("field", field).Str("trade", fmt.Sprintf("%+v", trade)).Msg("field not found in trade")
		return []float64{trade.Price, t}
	}
}

func TradesData(target string, trades []coinmodel.Trade) [][]float64 {

	datapoints := make([][]float64, len(trades))

	for i, trade := range trades {
		datapoints[i] = TradeData(target, trade)
	}

	return datapoints

}

func PositionData(position local.TrackedPosition) [][]float64 {
	points := make([][]float64, 0)

	ot := Time(position.Open)
	points = append(points, []float64{position.Position.OpenPrice, ot})

	if position.Position.CurrentPrice > 0 {
		it := Time(position.Close)
		points = append(points, []float64{position.Position.CurrentPrice, it})
	}
	return points
}

func Time(t time.Time) float64 {
	return float64(t.Unix()) * 1000
}
