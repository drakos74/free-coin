package processor

import (
	"math"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const tradeProcessorName = "trade"

// Trades is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
func Trader(client api.TradeClient, user api.UserInterface) api.Processor {

	//configuration := make(map[api.Coin]*openConfig)

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", tradeProcessorName).Msg("closing' strategy")
			close(out)
		}()

		for trade := range in {

			metrics.Observer.IncrementTrades(string(trade.Coin), tradeProcessorName)

			// decide if we open a new position
			for _, dd := range defaultDurations {
				rsi := MetaRSI(trade, dd)
				bucketView := MetaBucket(trade, dd)
				predictions := MetaStatsPredictions(trade, dd)
				if len(predictions) > 0 && rsi.RSI > 0 && math.Abs(bucketView.price.EMADiff) > 10 {
					// check if we should make a buy order
				}
			}

			out <- trade
		}
		log.Info().Str("processor", tradeProcessorName).Msg("closing processor")
	}
}

type openConfig struct {
	coin   model.Coin
	volume float64
}

var defaultOpenConfig = map[model.Coin]openConfig{
	model.BTC: {
		coin:   model.BTC,
		volume: 0.01,
	},
	model.ETH: {
		coin:   model.ETH,
		volume: 0.1,
	},
}
