package stats

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

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

// MultiStats allows the user to start and stop their own stats processors from the commands channel
// TODO : split responsibilities of this class to make things more clean and re-usable
func MultiStats(user api.User, configs ...Config) api.Processor {

	if len(configs) == 0 {
		configs = loadDefaults()
	}

	stats, err := newStats(configs)
	if err != nil {
		log.Error().Err(err).Str("processor", ProcessorName).Msg("could not init processor")
		return processor.Void(ProcessorName)
	}

	//cmdSample := "?notify [Time in minutes] [start/stop]"

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
			if _, ok := stats.configs[trade.Coin]; !ok {
				stats.configs[trade.Coin] = stats.configs[""]
			}
			for duration, cfg := range stats.configs[trade.Coin] {
				k := sKey{
					duration: duration,
					coin:     trade.Coin,
				}
				// we got the config, check if we need something to do in the windows
				stats.start(k)
				// push the trade data to the stats collector window
				if buckets, ok := stats.push(k, trade); ok {
					values, indicators, last := extractFromBuckets(buckets,
						group(getPriceRatio, cfg.order))
					// count the occurrences
					predictions, status := stats.add(k, values[0][len(values[0])-1])
					if trade.Live {
						aggregateStats := coinmath.NewAggregateStats(indicators)
						trade.Signal = model.Signal{
							Type: "TradeSignal",
							Value: TradeSignal{
								Coin:           trade.Coin,
								Price:          trade.Price,
								Time:           trade.Time,
								Duration:       k.duration,
								Predictions:    predictions,
								AggregateStats: aggregateStats,
							},
						}
						if user != nil {
							// TODO : add tests for this
							user.Send(api.Public,
								api.NewMessage(createStatsMessage(last, values, aggregateStats, predictions, status, trade, cfg)).
									ReferenceTime(trade.Time), nil)
						}
						// TODO : expose in metrics
						//fmt.Println(fmt.Sprintf("buffer = %+v", buffer))
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
	Predictions    map[string]buffer.Prediction
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
		fs := "%d"
		if f > 0 {
			fs = fmt.Sprintf("+%s", fs)
		} else if f < 0 {
			fs = fmt.Sprintf("-%s", fs)
		}
		s := fmt.Sprintf(fs, v)
		return s
	}
}

func trackUserActions(user api.User, stats *statsCollector) {
	for command := range user.Listen("stats", "?n") {
		var duration int
		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?n", "?notify"),
			api.Int(&duration),
			api.OneOf(&action, "start", "stop", ""),
		)
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}
		//timeDuration := Time.Duration(Duration) * Time.Minute

		switch action {
		case "":
			// TODO : return the currently running stats processes
			//case "start":
			//	// TODO : re-enable this functionality.
			//	if stats.hasOrAddDuration(timeDuration) {
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

func createStatsMessage(last windowView, values [][]string, aggregateStats coinmath.AggregateStats, predictions map[string]buffer.Prediction, status buffer.Status, p *model.Trade, cfg windowConfig) string {
	// TODO : make the trigger arguments more specific to current stats statsCollector

	// format the Predictions.
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
	for k, v := range predictions {
		keySlice := strings.Split(k, ":")
		valueSlice := strings.Split(v.Value, ":")
		emojiKeySlice := emoji.MapToSymbols(keySlice)
		emojiValueSlice := emoji.MapToSymbols(valueSlice)
		pp[i] = fmt.Sprintf("%s -> %s ( %.2f | %.2f | %v | %d | %d) ",
			strings.Join(emojiKeySlice, " "),
			strings.Join(emojiValueSlice, " "),
			v.Probability,
			1/float64(v.Options),
			v.Sample,
			v.Groups,
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
	ps := fmt.Sprintf("%s|%.0fm: %s ...",
		p.Coin,
		cfg.duration.Minutes(),
		strings.Join(emojiValues, " "))

	// last bucket Price details
	move := emoji.MapToSentiment(last.price.Ratio)
	mp := fmt.Sprintf("%s %s§ ratio:%.2f stdv:%.2f ema:%.2f",
		move,
		coinmath.Format(p.Price),
		last.price.Ratio*100,
		last.price.StdDev,
		last.price.EMADiff)

	// ignore the values smaller than '0' just to be certain
	vol := emoji.MapNumber(coinmath.O2(math.Round(last.volume.Diff)))
	mv := fmt.Sprintf("%s %s§ ratio:%.2f stdv:%.2f ema:%.2f",
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
		// Predictions details
		strings.Join(pp, "\n "))

}
