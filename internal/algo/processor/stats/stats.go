package stats

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/emoji"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	ProcessorName = "stats"
)

func trackUserActions(user api.User, stats *statsCollector) {
	for command := range user.Listen("stats", "?n") {
		var duration int
		var coin string
		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?n", "?notify"),
			api.Any(&coin),
			api.Int(&duration),
			api.OneOf(&action, "start", "stop", ""),
		)
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}
		c := model.Coin(coin)
		d := time.Duration(duration) * time.Minute
		k := processor.NewKey(c, d)
		switch action {
		case "":
			if w, ok := stats.windows[k]; ok {
				sendWindowConfig(user, k, w)
			} else {
				for k, w := range stats.windows {
					sendWindowConfig(user, k, w)
				}
			}
			// TODO : re-enable this functionality.
			//case "start":
			//	if stats.windows(timeDuration) {
			//		api.Reply(api.Private, user,
			//			api.NewMessage(fmt.Sprintf("notify window for '%v' mins is running ... please be patient", timeDuration.Minutes())).
			//				ReplyTo(command.ID), nil)
			//		continue
			//	} else {
			//		api.Reply(api.Private, user,
			//			api.NewMessage(fmt.Sprintf("started notify window %v", command.Content)).
			//				ReplyTo(command.ID), nil)
			//	}
			//case "stop":
			//	stats.stop(timeDuration)
			//	api.Reply(api.Private, user,
			//		api.NewMessage(fmt.Sprintf("removed notify window for '%v' mins", timeDuration.Minutes())).
			//			ReplyTo(command.ID), nil)
		}
	}
}

func sendWindowConfig(user api.User, k processor.Key, w *window) {
	for _, cfg := range w.c.Config {
		api.Reply(api.Private,
			user,
			api.NewMessage(fmt.Sprintf("%s|%v %v -> %v (%d)",
				k.Coin,
				k.Duration,
				cfg.LookBack,
				cfg.LookAhead,
				w.c.Status.Count),
			), nil)
	}
}

// MultiStats allows the user to start and stop their own stats processors from the commands channel
// TODO : split responsibilities of this class to make things more clean and re-usable
func MultiStats(registry storage.Registry, user api.User, configs map[model.Coin]map[time.Duration]processor.Config) api.Processor {

	stats, err := newStats(registry, configs)
	if err != nil {
		log.Error().Err(err).Str("processor", ProcessorName).Msg("could not init processor")
		return processor.Void(ProcessorName)
	}

	go trackUserActions(user, stats)

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {

		defer func() {
			log.Info().Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for trade := range in {
			metrics.Observer.IncrementTrades(string(trade.Coin), ProcessorName)
			// set up the config for the coin if it s not there.
			// use "" as default ... if its missing i guess we ll fail hard at some point ...
			for duration, cfg := range stats.configs[trade.Coin] {
				k := processor.NewKey(trade.Coin, duration)
				// push the trade data to the stats collector window
				if buckets, ok := stats.push(k, trade); ok {
					values, indicators, last := extractFromBuckets(buckets, group(getPriceRatio, cfg.Order.Exec))
					// count the occurrences
					// TODO : give more weight to the more recent values that come in
					// TODO : old values might destroy our statistics and be irrelevant
					predictions, status := stats.add(k, values[0][len(values[0])-1])
					if trade.Live {
						if trade.Signals == nil {
							trade.Signals = make([]model.Signal, 0)
						}
						aggregateStats := coinmath.NewAggregateStats(indicators)
						// Note there is one trade signal per key (coin,duration) pair
						trade.Signals = append(trade.Signals, model.Signal{
							Type: "TradeSignal",
							Value: TradeSignal{
								Coin:           trade.Coin,
								Price:          trade.Price,
								Time:           trade.Time,
								Duration:       k.Duration,
								Predictions:    predictions,
								AggregateStats: aggregateStats,
							},
						})
						if user != nil && cfg.Notify.Stats {
							// TODO : add tests for this
							user.Send(api.Public,
								api.NewMessage(createStatsMessage(last, values, aggregateStats, predictions, status, trade, cfg)).
									ReferenceTime(trade.Time), nil)
						}
						// TODO : expose in metrics
					}
				}
			}
			out <- trade
		}
	}
}

type TradeSignal struct {
	Coin           model.Coin
	Price          float64
	Time           time.Time
	Duration       time.Duration
	Predictions    map[buffer.Sequence]buffer.Predictions
	AggregateStats coinmath.AggregateStats
}

type windowView struct {
	price  buffer.TimeWindowView
	volume buffer.TimeWindowView
}

func getPriceRatio(b windowView) float64 {
	return b.price.Ratio
}

// extractFromBuckets extracts from the given buckets the needed values
func extractFromBuckets(ifc interface{}, format ...func(b windowView) string) ([][]string, *coinmath.Indicators, windowView) {
	s := reflect.ValueOf(ifc)
	pp := make([][]string, s.Len())
	var last windowView
	stream := coinmath.NewIndicators()
	l := s.Len()
	for j, f := range format {
		pp[j] = make([]string, l)
		for i := 0; i < l; i++ {
			b := s.Index(i).Interface().(windowView)
			last = b
			pp[j][i] = f(b)
			stream.Add(b.price.Diff)
		}
	}
	return pp, stream, last
}

func group(extract func(b windowView) float64, group func(f float64) int) func(b windowView) string {
	return func(b windowView) string {
		f := extract(b)
		v := group(f)
		// TODO :maybe we start counting values only over a limit
		s := strconv.FormatInt(int64(v), 10)
		return s
	}
}

// TODO : Removing the ordering capabilities on the stats processor for now (?)
//func openPositionTrigger(p *model.Trade, client api.Exchange) api.TriggerFunc {
//	return func(command api.Command) (string, error) {
//		// TODO: pars optionally the volume
//		var t model.Type
//		switch command.Content {
//		case "buy":
//			t = model.Buy
//		case "sell":
//			t = model.Sell
//		default:
//			return "[error]", fmt.Errorf("unknown command: %s", command.Content)
//		}
//		if vol, ok := defaultOpenConfig[p.Coin]; ok {
//			group := model.NewOrder(p.Coin).
//				WithLeverage(model.L_5).
//				WithVolume(vol.volume).
//				WithType(t).
//				Market().
//				Create()
//			return fmt.Sprintf("opened position for %s", p.Coin), client.OpenOrder(group)
//
//		} else {
//			return "could not open position", fmt.Errorf("no pre-defined volume for %s", p.Coin)
//		}
//	}
//}

func createStatsMessage(last windowView, values [][]string, aggregateStats coinmath.AggregateStats, predictions map[buffer.Sequence]buffer.Predictions, status buffer.Status, p *model.Trade, cfg processor.Config) string {
	// TODO : make the trigger arguments more specific to current stats statsCollector

	// format the PredictionList.
	pp := make([]string, len(predictions)+1)
	samples := make([]string, len(status.Samples))
	k := 0
	for _, sample := range status.Samples {
		var ss int
		for _, smpl := range sample {
			ss += smpl.Events
		}
		samples[k] = fmt.Sprintf("%d : %d", ss, len(sample))
		k++
	}
	pp[0] = fmt.Sprintf("%+v -> %s", status.Count, strings.Join(samples, " | "))
	i := 1
	for _, v := range predictions {
		pp[i] = fmt.Sprintf("%s -> %v [ %d : %d : %d ] ",
			emoji.Sequence(v.Key),
			emoji.PredictionList(v.Values),
			len(v.Values),
			v.Sample,
			v.Count,
		)
		i++
	}

	// format the past values
	emojiValues := make([]string, len(values[0]))
	for j := 0; j < len(values[0]); j++ {
		emojiValues[j] = emoji.MapToSymbol(values[0][j])
	}

	// stats processor details
	ps := fmt.Sprintf("%s|%dm: %s ...",
		p.Coin,
		cfg.Duration,
		strings.Join(emojiValues, " "))

	// last bucket Price details
	move := emoji.MapToSentiment(last.price.Ratio)
	mp := fmt.Sprintf("%s %s€ ratio:%.2f stdv:%.2f ema:%.2f",
		move,
		coinmath.Format(p.Price),
		last.price.Ratio*100,
		last.price.StdDev,
		last.price.EMADiff)

	// ignore the values smaller than '0' just to be certain
	vol := emoji.MapNumber(coinmath.O2(math.Round(last.volume.Diff)))
	mv := fmt.Sprintf("%s %s€ ratio:%.2f stdv:%.2f ema:%.2f",
		vol,
		coinmath.Format(last.volume.Value),
		last.volume.Ratio,
		last.volume.StdDev,
		last.volume.EMADiff)

	// bucket collector details
	st := fmt.Sprintf("rsi:%d ersi:%d ema:%.2f (%d)",
		aggregateStats.RSI,
		aggregateStats.ERSI,
		100*(aggregateStats.EMA-last.price.Value)/last.price.Value,
		aggregateStats.Sample)

	// TODO : make this formatting easier
	// format the status message for the processor.
	return fmt.Sprintf("%s\n %s\n %s\n %s\n %s",
		ps,
		mp,
		mv,
		st,
		// PredictionList details
		strings.Join(pp, "\n "))

}
