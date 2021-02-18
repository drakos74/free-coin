package processor

import (
	"math"

	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/rs/zerolog/log"
)

const tradeProcessorName = "trade"

// Trades is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
func Trader(client model.TradeClient, user model.UserInterface) api.Processor {

	//configuration := make(map[api.Coin]*openConfig)

	return func(in <-chan *api.Trade, out chan<- *api.Trade) {
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
				if len(predictions) > 0 && rsi.RSI > 0 && math.Abs(bucketView.EMADiff) > 10 {
					// check if we should make a buy order
				}
			}

			out <- trade
		}
		log.Info().Str("processor", tradeProcessorName).Msg("closing processor")
	}
}

type openConfig struct {
	coin   api.Coin
	volume float64
}

var defaultOpenConfig = map[api.Coin]openConfig{
	api.BTC: {
		coin:   api.BTC,
		volume: 0.01,
	},
	api.ETH: {
		coin:   api.ETH,
		volume: 0.1,
	},
}
