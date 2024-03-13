package processor

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	this_time "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

const file_size int = 10000

// History is a grouping processor for past trades.
func History(name string, coin model.Coin, dir string) api.Processor {

	interval := 5 * time.Minute

	trades := make([]model.TradeSignal, file_size)
	index := 0
	first := time.Now()
	last := time.Now()

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {

	}

	coinString := string(coin)
	intervalFloat := interval.Minutes()

	return ProcessBufferedWithClose(name, interval, false, func(trade *model.TradeSignal) error {
		if index == 0 {
			first = trade.Meta.Time
		}
		last = trade.Meta.Time
		trades[index] = *trade
		index++

		metrics.Observer.IncrementTrades(coinString, "history", "receive")
		f, _ := strconv.ParseFloat(last.Format("20060102.1504"), 64)
		metrics.Observer.NoteLag(f, coinString, "history", "last")

		if index >= file_size {
			name := fileName(coinString, intervalFloat, first, last)
			file, err := os.Create(fmt.Sprintf("%s/%s.json", dir, name))
			// flush and create new
			data, err := json.Marshal(trades)
			if err != nil {
				panic(fmt.Sprintf("could not marshall data: %+v", err))
			}
			_, err = file.Write(data)
			if err != nil {
				panic(fmt.Sprintf("could not save file: %+v", err))
			}
			err = file.Close()
			if err != nil {
				panic(fmt.Sprintf("could not close file: %+v", err))
			}
			log.Info().
				Timestamp().
				Int("size", file_size).
				Str("file", name).
				Msg("written file")
			trades = make([]model.TradeSignal, file_size)
			index = 0
		}

		return nil
	}, func() {}, Deriv())
}

func fileName(coin string, interval float64, first, last time.Time) string {
	return fmt.Sprintf("%s_%.0f_%s_%s", coin, interval, this_time.ToString(first), this_time.ToString(last))
}
