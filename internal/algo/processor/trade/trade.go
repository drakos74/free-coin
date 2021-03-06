package trade

import (
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

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
	for command := range user.Listen("trader", "?Type") {
		var coin string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?Type", "?trade"),
			api.Any(&coin),
		)
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}

		c := model.Coin(coin)

		// return the current configs
		for d, cfg := range trader.getAll(c) {
			for _, strategy := range cfg.Strategies {
				user.Send(api.Private,
					api.NewMessage(fmt.Sprintf("%s %dm[%v] ( p:%f | s:%d | t:%f )",
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
					k := processor.NewKey(ts.Coin, ts.Duration)
					// init the configuration for this pair of a coin and duration.
					// TODO : use an internal state like for the stats processor
					// we got a trade signal
					predictions := ts.Predictions
					if cfg, ok := trader.get(k); ok && len(predictions) > 0 {
						// check if we should make a buy order
						pairs := evaluate(ts, cfg.Strategies)
						// act here ... once for every trade signal only once per coin
						if len(pairs) > 0 {
							// note the first pair should have the highest probability !!!
							// so we should probably just take that ...
							// TODO : confirm that in tests
							pair := pairs[0]
							fmt.Println(fmt.Sprintf("pair = %+v", pair))
							vol := getVolume(ts.Price, pair.Open)
							// we will make only one order from all the pairs ...
							cid := processor.Correlate(ts.Coin, ts.Duration, pair.Strategy)
							order := model.NewOrder(ts.Coin, cid).
								SubmitTime(pair.Time).
								WithLeverage(model.L_5).
								WithVolume(vol).
								WithType(pair.Type).
								Market().
								Create()
							// TODO : save this log into our processor
							pair.ID = order.ID
							err := trader.logger.Put(storage.K{
								Pair:  string(ts.Coin),
								Label: ProcessorName,
							}, pair)
							log.Info().
								Str("ID", order.ID).
								Err(err).
								Str("pair", fmt.Sprintf("%+v", pair)).
								Float64("Price", pair.Price).
								Str("Coin", string(ts.Coin)).
								Msg("open position")
							// signal to the position processor that there should be a new one
							block.Action <- api.NewAction(model.OrderKey).ForCoin(ts.Coin).WithContent(order).Create()
							// wait for the position processor to acknowledge the update
							<-block.ReAction
							api.Reply(api.Private, user, api.
								NewMessage(createPredictionMessage(pair)).
								AddLine(fmt.Sprintf("open %s %s ( %.2f | %.2f ) [%s]",
									emoji.MapType(pair.Type),
									ts.Coin,
									vol,
									pair.Price,
									order.ID,
								)), nil)
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
	pp := fmt.Sprintf("(%.2f | %d)", pair.Probability, pair.Sample)
	line := fmt.Sprintf("%s | %s -> %s %s", pair.Label, kk, strings.Join(vv, " | "), pp)
	return line
}

type PredictionPair struct {
	ID          string            `json:"id"`
	Price       float64           `json:"price"`
	Time        time.Time         `json:"time"`
	Confidence  float64           `json:"confidence"`
	Open        float64           `json:"open"`
	Strategy    string            `json:"strategy"`
	Label       string            `json:"label"`
	Key         buffer.Sequence   `json:"key"`
	Values      []buffer.Sequence `json:"values"`
	Probability float64           `json:"probability"`
	Sample      int               `json:"sample"`
	Type        model.Type        `json:"type"`
}

type predictionsPairs []PredictionPair

// for sorting predictions
func (p predictionsPairs) Len() int           { return len(p) }
func (p predictionsPairs) Less(i, j int) bool { return p[i].Probability < p[j].Probability }
func (p predictionsPairs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
