package processor

import (
	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/rs/zerolog/log"
)

// Trades is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
func Trader(client model.TradeClient, user model.UserInterface) api.Processor {

	return func(in <-chan *api.Trade, out chan<- *api.Trade) {
		defer func() {
			log.Info().Msg("closing 'MultiStats' strategy")
			close(out)
		}()

		for trade := range in {

			for _, dd := range defaultDurations {
				rsi := MetaRSI(trade, dd)
				predictions := MetaStatsPredictions(trade, dd)
				if len(predictions) > 0 && rsi.RSI > 0 {
					// check if we should make a buy order
				}
			}

		}
	}

}
