package trade

import (
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/algo/processor/stats"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const ProcessorName = "trade"

func trackUserActions(user api.User, trader *trader) {
	for command := range user.Listen("trader", "?t") {
		var duration int
		var coin string
		var probability float64
		var sample int
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?t", "?trade"),
			api.Any(&coin),
			api.Int(&duration),
			api.Float(&probability),
			api.Int(&sample),
		)
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}
		timeDuration := time.Duration(duration) * time.Minute

		c := model.Coin(coin)

		if probability > 0 {
			d, newConfig := trader.set(processor.NewKey(c, timeDuration), probability, sample)
			api.Reply(api.Private, user, api.NewMessage(fmt.Sprintf("%s %dm probability:%f sample:%d",
				c,
				int(d.Minutes()),
				newConfig.MinProbability,
				newConfig.MinSample)), nil)
		} else {
			// return the current configs
			for d, cfg := range trader.getAll(c) {
				user.Send(api.Private,
					api.NewMessage(fmt.Sprintf("%s %dm probability:%f sample:%d",
						c,
						int(d.Minutes()),
						cfg.MinProbability,
						cfg.MinSample)), nil)
			}
		}
	}
}

// Trade is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
// client is the exchange client used to open orders
// user is the user interface for interacting with the user
// block is the internal synchronisation mechanism used to make sure requests to the client are processed before proceeding
func Trade(client api.Exchange, user api.User, block api.Block, configs ...Config) api.Processor {

	if len(configs) == 0 {
		configs = loadDefaults()
	}

	trader := newTrader(configs...)

	go trackUserActions(user, trader)

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for trade := range in {
			//metrics.Observer.IncrementTrades(string(trade.Coin), ProcessorName)
			// TODO : check also Active
			if trade == nil || !trade.Live || trade.Signals == nil {
				out <- trade
				continue
			}

			pairs := make(map[processor.Key][]predictionPair)
			for i, signal := range trade.Signals {
				if ts, ok := signal.Value.(stats.TradeSignal); ok {
					k := processor.NewKey(ts.Coin, ts.Duration)
					// init the configuration for this pair of a coin and duration.
					trader.init(k)
					// TODO : use an internal state like for the stats processor
					// we got a trade signal
					predictions := ts.Predictions
					cfg, ok := trader.get(k)
					if len(predictions) > 0 && ok {
						// check if we should make a buy order
						for k, p := range predictions {
							if p.Probability >= cfg.MinProbability && p.Sample >= cfg.MinSample {
								// we know it s a good prediction. Lets check the value
								v := p.Value
								vv := strings.Split(v, ":")
								// TODO : now it takes a random t if there are more matches
								if t := cfg.contains(vv); t > 0 {
									kk := processor.Key{
										Coin:     ts.Coin,
										Duration: ts.Duration,
										Index:    i,
									}

									if _, ok := pairs[kk]; !ok {
										pairs[kk] = make([]predictionPair, 0)
									}
									log.Info().
										Str("trade-ID", trade.ID).
										Str("prediction", fmt.Sprintf("%+v", p)).
										Str("key", fmt.Sprintf("%+v", kk)).
										Time("time", trade.Time).
										Msg("adding open order pair")
									pairs[kk] = append(pairs[kk], predictionPair{
										price:       ts.Price,
										openValue:   cfg.Value,
										t:           t,
										key:         k,
										value:       v,
										probability: p.Probability,
										base:        1 / float64(p.Options),
										sample:      p.Sample,
										label:       p.Label,
									})
								}
							}
						}
					}
				}
			}
			var cancel bool
			var gotAction bool
			var coin model.Coin
			var vol float64
			var pair predictionPair
			for k, pp := range pairs {
				for _, p := range pp {
					gotAction = true
					// decide to do only one action
					coin = k.Coin
					vol = getVolume(p.price, p.openValue)

					// check if we have mixed signals
					if pair.t != model.NoType && pair.t != p.t {
						cancel = true
					}
					pair = p
				}
			}
			if !cancel && gotAction {
				// we will make only one order from all the pairs ...
				order := model.NewOrder(coin).
					WithLeverage(model.L_5).
					WithVolume(vol).
					WithType(pair.t).
					Market().
					Create()
				log.Info().
					Str("ID", order.ID).
					Float64("price", pair.price).
					Str("Coin", string(coin)).
					Msg("open position")
				err := client.OpenOrder(order)
				block.Action <- api.Action{}
				api.Reply(api.Private, user, api.
					NewMessage(createPredictionMessage(pair)).
					AddLine(fmt.Sprintf("open %v %f %s at %f", emoji.MapType(pair.t), vol, coin, pair.price)), err)
				<-block.ReAction
			} else {
				if gotAction {
					log.Warn().Bool("cancel", cancel).
						Bool("action", gotAction).
						Str("coin", string(coin)).
						Msg("got mixed signals")
				}
			}

			out <- trade
		}
	}
}

func createPredictionMessage(pair predictionPair) string {
	kk := emoji.MapToSymbols(strings.Split(pair.key, ":"))
	vv := emoji.MapToSymbols(strings.Split(pair.value, ":"))
	pp := fmt.Sprintf("(%.2f | %.2f | %d)", pair.probability, pair.base, pair.sample)
	line := fmt.Sprintf("%s | %s -> %s %s", pair.label, strings.Join(kk, " "), strings.Join(vv, " "), pp)
	return line
}

type predictionPair struct {
	price       float64
	openValue   float64
	label       string
	key         string
	value       string
	probability float64
	base        float64
	sample      int
	t           model.Type
}
