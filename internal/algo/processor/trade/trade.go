package trade

import (
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/algo/processor/stats"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const ProcessorName = "trade"

func trackUserActions(user api.User, trader *trader) {
	for command := range user.Listen("trader", "?Type") {
		var coin string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?Type", "?trade"),
			api.Any(&coin),
		)
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage(processor.Audit(ProcessorName, "cmd error")).ReplyTo(command.ID), err)
			continue
		}

		c := model.Coin(coin)

		// return the current configs
		for d, cfg := range trader.getAll(c) {
			for _, strategy := range cfg.Strategies {
				user.Send(api.Private,
					api.NewMessage(processor.Audit(ProcessorName, "positions")).
						AddLine(fmt.Sprintf("%s %dm[%v] ( p:%f | s:%d | t:%f )",
							c,
							cfg.Duration,
							// TODO : we dont need both, but keeping them for debugging for now
							d,
							strategy.Probability,
							strategy.Sample,
							strategy.Threshold)), nil)
			}
		}
		// TODO : allow to edit based on the reply message
	}
}

// Trade is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
// client is the exchange client used to open orders
// user is the user interface for interacting with the user
// block is the internal synchronisation mechanism used to make sure requests to the client are processed before proceeding
func Trade(registry storage.Registry, user api.User, block api.Block, configs map[model.Coin]map[time.Duration]processor.Config) api.Processor {

	trader := newTrader(registry, configs)

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

			for _, signal := range trade.Signals {
				if ts, ok := signal.Value.(stats.TradeSignal); ok {
					// init the configuration for this pair of a coin and duration.
					// TODO : use an internal state like for the stats processor
					// we got a trade signal, let's see if we can get an action out of it
					if cfg, ok := trader.get(ts.Key); ok && len(ts.Predictions) > 0 {
						// check if we should make a buy order
						pairs := trader.evaluate(ts, cfg.Strategies)
						// act here ... once for every trade signal only once per coin
						if len(pairs) > 0 {
							// note the first pair should have the highest probability !!!
							// so we should probably just take that ...
							// TODO : confirm that in tests
							pair := pairs[0]
							k := ts.Key
							k.Strategy = pair.Strategy.Name
							vol := getVolume(ts.Price, pair.Strategy.Open.Value, pair.Confidence)
							// we will make only one order from all the pairs ...
							order := model.NewOrder(ts.Key.Coin).
								WithLeverage(model.L_5).
								WithVolume(vol).
								WithType(pair.Type).
								Market().
								Create()
							trackedOrder := model.TrackedOrder{
								Order: order,
								Key:   k,
								Time:  pair.Time,
							}
							// TODO : save this log into our processor
							pair.ID = order.ID
							err := trader.registry.Add(triggerKey(string(k.Coin)), pair)
							log.Info().
								Err(err).
								Str("ID", order.ID).
								Int("signals", len(trade.Signals)).
								Int("pairs", len(pairs)).
								Str("pair", fmt.Sprintf("%+v", pair)).
								Float64("Price", pair.Price).
								Str("Coin", string(k.Coin)).
								Msg("open position")
							// signal to the position processor that there should be a new one
							block.Action <- api.NewAction(model.OrderKey).ForCoin(k.Coin).WithContent(trackedOrder).Create()
							// wait for the position processor to acknowledge the update
							<-block.ReAction
							api.Reply(api.Private, user, api.
								NewMessage(processor.Audit(ProcessorName, createPredictionMessage(pair))).
								AddLine(fmt.Sprintf("open %s %s ( %.3f | %.2f )",
									emoji.MapType(pair.Type),
									k.Coin,
									vol,
									pair.Price,
								)).
								// TODO :remove this, it s for temporary debugging
								AddLine(fmt.Sprintf("signal-id : %s", pair.SignalID)), nil)
						}
					}
				}
			}
			out <- trade
		}
	}
}

func createPredictionMessage(pair PredictionPair) string {
	kk := emoji.Sequence(pair.Key)
	vv := make([]string, 0)
	for _, pv := range pair.Values {
		vv = append(vv, emoji.Sequence(pv))
	}
	pp := fmt.Sprintf("( %.2f | %d )", pair.Probability, pair.Sample)
	line := fmt.Sprintf("%s | %.2f | %s -> %s %s",
		pair.Label,
		pair.Confidence,
		kk,
		strings.Join(vv, " | "), pp)
	return line
}
