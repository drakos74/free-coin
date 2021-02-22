package processor

import (
	"fmt"
	"strings"

	"github.com/drakos74/free-coin/internal/emoji"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const tradeProcessorName = "trade"

// Trade is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
func Trade(client api.Exchange, user api.User, signal <-chan api.Signal) api.Processor {

	//configuration := make(map[api.Coin]*openConfig)

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", tradeProcessorName).Msg("closing' strategy")
			close(out)
		}()

		for {
			select {
			case trade := <-in:
				metrics.Observer.IncrementTrades(string(trade.Coin), tradeProcessorName)
				if !trade.Live {
					out <- trade
					continue
				}
				out <- trade
			case s := <-signal:
				if ts, ok := s.Value.(tradeSignal); ok {
					// we got a trade signal
					predictions := ts.predictions
					if len(predictions) > 0 {
						// check if we should make a buy order
						var buy bool
						var sell bool
						pairs := make([]predictionPair, 0)
						for k, p := range predictions {
							v := p.Value
							if strings.HasPrefix(v, "+1") ||
								strings.HasPrefix(v, "+0") ||
								strings.HasPrefix(v, "+2:+1") ||
								strings.HasPrefix(v, "+2:+2") {
								if p.Probability > 0.55 && p.Sample > 10 {
									buy = true
									pairs = append(pairs, predictionPair{
										key:   k,
										value: v,
									})
								}
							} else if strings.HasPrefix(v, "-1") ||
								strings.HasPrefix(v, "-0") ||
								strings.HasPrefix(v, "-2:-1") ||
								strings.HasPrefix(v, "-2:-2") {
								if p.Probability > 0.55 && p.Sample > 10 {
									sell = true
									pairs = append(pairs, predictionPair{
										key:   k,
										value: v,
									})
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
							if vol, ok := defaultOpenConfig[ts.coin]; ok {
								log.Info().
									Str("predictions", fmt.Sprintf("%+v", predictions)).
									Str("coin", string(ts.coin)).
									Msg("open order")
								// TODO : print the prediction in the reply message
								err := client.OpenOrder(model.NewOrder(ts.coin).
									WithLeverage(model.L_5).
									WithVolume(vol.volume).
									WithType(t).
									Market().
									Create())
								// TODO : combine with the trades to know of the price
								api.Reply(api.Private, user, api.NewMessage(createPredictionMessage(pairs)).AddLine(fmt.Sprintf("open %v %f %s at %f", t, vol.volume, ts.coin, ts.price)), err)
							}
						} else if buy && sell {
							log.Warn().Bool("buy", buy).Bool("sell", sell).Msg("inconclusive buy signal")
						}
					}
				}
			}
		}
		log.Info().Str("processor", tradeProcessorName).Msg("closing processor")
	}
}

func createPredictionMessage(pairs []predictionPair) string {
	lines := make([]string, len(pairs))
	for i, pair := range pairs {
		kk := emoji.MapToSymbols(strings.Split(pair.key, ":"))
		vv := emoji.MapToSymbols(strings.Split(pair.value, ":"))
		lines[i] = fmt.Sprintf("%s -> %s", strings.Join(kk, " "), strings.Join(vv, " "))
	}
	return strings.Join(lines, "\n")
}

type predictionPair struct {
	key   string
	value string
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
