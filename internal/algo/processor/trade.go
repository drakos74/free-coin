package processor

import (
	"fmt"
	"strings"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const tradeProcessorName = "trade"

// Trade is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
func Trade(client api.Exchange, user api.User) api.Processor {

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
				//stats := MetaIndicators(trade, dd)
				//bucketView := MetaBucket(trade, dd)
				predictions := MetaStatsPredictions(trade, dd)
				if len(predictions) > 0 {
					// check if we should make a buy order
					var buy bool
					var sell bool
					for k, p := range predictions {
						if strings.HasPrefix(k, "+1") ||
							strings.HasPrefix(k, "+0") ||
							strings.HasPrefix(k, "+2:+1") ||
							strings.HasPrefix(k, "+2:+2") {
							if p.Probability > 0.55 && p.Sample > 10 {
								buy = true
							}
						} else if strings.HasPrefix(k, "-1") ||
							strings.HasPrefix(k, "-0") ||
							strings.HasPrefix(k, "-2:-1") ||
							strings.HasPrefix(k, "-2:-2") {
							if p.Probability > 0.55 && p.Sample > 10 {
								sell = true
							}
						}
					}
					if buy != sell {
						t := model.NoType
						if buy {
							t = model.Buy
						} else if sell {
							t = model.Sell
						}
						if vol, ok := defaultOpenConfig[trade.Coin]; ok {
							err := client.OpenOrder(model.NewOrder(trade.Coin).
								WithLeverage(model.L_5).
								WithVolume(vol.volume).
								WithType(t).
								Market().
								Create())
							api.Reply(api.Private, user, api.NewMessage(fmt.Sprintf("open %v %f %s at %f", t, vol.volume, trade.Coin, trade.Price)), err)
						}
					} else if buy && sell {
						log.Warn().Bool("buy", buy).Bool("sell", sell).Msg("inconclusive buy signal")
					}
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
		volume: 0.3,
	},
	model.LINK: {
		coin:   model.LINK,
		volume: 15,
	},
	model.DOT: {
		coin:   model.DOT,
		volume: 15,
	},
	model.XRP: {
		coin:   model.XRP,
		volume: 1000,
	},
}
