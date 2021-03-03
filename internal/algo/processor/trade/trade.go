package trade

import (
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/buffer"

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
			for _, signal := range trade.Signals {
				if ts, ok := signal.Value.(stats.TradeSignal); ok {
					k := processor.NewKey(ts.Coin, ts.Duration)
					// init the configuration for this pair of a coin and duration.
					trader.init(k)
					// TODO : use an internal state like for the stats processor
					// we got a trade signal
					predictions := ts.Predictions
					if cfg, ok := trader.get(k); ok && len(predictions) > 0 {
						// check if we should make a buy order
						pairs[processor.Key{
							Coin:     ts.Coin,
							Duration: ts.Duration,
						}] = cfg.evaluate(ts)
					}
				}
			}
			//fmt.Println(fmt.Sprintf("pairs = %+v", pairs))
			if len(pairs) > 0 {
				log.Info().
					Int("pairs", len(pairs)).
					Msg("trade action")
				var cancel bool
				var gotAction bool
				var coin model.Coin
				var vol float64
				var pair predictionPair
				for k, pp := range pairs {
					if len(pp) > 0 {
						// take the first one ...
						p := pp[0]
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
				if !cancel && gotAction && pair.t != model.NoType {
					// we will make only one order from all the pairs ...
					order := model.NewOrder(coin).
						WithLeverage(model.L_5).
						WithVolume(vol).
						WithType(pair.t).
						Market().
						Create()
					// TODO : send the open request to position processor,
					// TODO : so that position is responsible for end2end order management
					txIDs, err := client.OpenOrder(order)
					log.Info().
						Str("ID", order.ID).
						Err(err).
						Str("pair", fmt.Sprintf("%+v", pair)).
						Float64("price", pair.price).
						Str("Coin", string(coin)).
						Strs("tx-ids", txIDs).
						Msg("open position")
					// signal to the position processor that there should be a new one
					block.Action <- api.NewAction("open").ForCoin(coin).Create()
					api.Reply(api.Private, user, api.
						NewMessage(createPredictionMessage(pair)).
						AddLine(fmt.Sprintf("open %s %s ( %.2f | %.2f )",
							emoji.MapType(pair.t),
							coin,
							vol,
							pair.price,
						)), err)
					// wait for the position processor to acknowledge the update
					<-block.ReAction
				} else {
					if gotAction {
						log.Warn().Bool("cancel", cancel).
							Str("pairs", fmt.Sprintf("%+v", pairs)).
							Bool("action", gotAction).
							Str("coin", string(coin)).
							Msg("got mixed signals")
					}
				}
			}
			out <- trade
		}
	}
}

func createPredictionMessage(pair predictionPair) string {
	kk := emoji.MapToSymbols(strings.Split(pair.key, ":"))
	vv := make([]string, 0)
	for _, pv := range pair.values {
		vv = append(vv, buffer.ToStringSymbols(pv))
	}
	pp := fmt.Sprintf("(%.2f | %d)", pair.probability, pair.sample)
	line := fmt.Sprintf("%s | %s -> %s %s", pair.label, strings.Join(kk, " "), strings.Join(vv, " "), pp)
	return line
}

type predictionPair struct {
	price       float64
	openValue   float64
	name        string
	label       string
	key         string
	values      []string
	probability float64
	sample      int
	t           model.Type
}

type predictionsPairs []predictionPair

// for sorting predictions
func (p predictionsPairs) Len() int           { return len(p) }
func (p predictionsPairs) Less(i, j int) bool { return p[i].probability < p[j].probability }
func (p predictionsPairs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
