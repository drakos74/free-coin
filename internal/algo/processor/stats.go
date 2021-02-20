package processor

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/emoji"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	statsProcessorName = "stats"
)

var (
	defaultDurations = []time.Duration{10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour}
)

type windowConfig struct {
	duration     time.Duration
	historySizes int64
	counterSizes []buffer.HMMConfig
}

func newWindowConfig(duration time.Duration) windowConfig {
	return windowConfig{
		duration:     duration,
		historySizes: 14,
		counterSizes: []buffer.HMMConfig{
			{PrevSize: 3, TargetSize: 1},
			{PrevSize: 4, TargetSize: 2},
			{PrevSize: 5, TargetSize: 3},
		},
	}
}

type window struct {
	w *buffer.HistoryWindow
	c *buffer.HMM
}

type state struct {
	configs map[time.Duration]windowConfig
	windows map[time.Duration]map[model.Coin]window
}

func trackUserActions(user api.UserInterface, stats *state) {
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
		timeDuration := time.Duration(duration) * time.Minute

		switch action {
		case "":
			// TODO : return the currently running stats processes
		case "start":
			if _, ok := stats.configs[timeDuration]; ok {
				api.Reply(api.Private, user,
					api.NewMessage(fmt.Sprintf("notify window for '%v' mins is running ... please be patient", timeDuration.Minutes())).
						ReplyTo(command.ID), nil)
				continue
			}
			// TODO : decide how to handle the historySizes, especially in combination with the counterSizes.
			stats.configs[timeDuration] = newWindowConfig(timeDuration)
			stats.windows[timeDuration] = make(map[model.Coin]window)
			api.Reply(api.Private, user,
				api.NewMessage(fmt.Sprintf("started notify window %v", command.Content)).
					ReplyTo(command.ID), nil)
		case "stop":
			delete(stats.configs, timeDuration)
			delete(stats.windows, timeDuration)
			api.Reply(api.Private, user,
				api.NewMessage(fmt.Sprintf("removed notify window for '%v' mins", timeDuration.Minutes())).
					ReplyTo(command.ID), nil)
		}
	}
}

func openPositionTrigger(p *model.Trade, client api.TradeClient) api.TriggerFunc {
	return func(command api.Command) (string, error) {
		var t model.Type
		switch command.Content {
		case "buy":
			t = model.Buy
		case "sell":
			t = model.Sell
		default:
			return "[error]", fmt.Errorf("unknown command: %s", command.Content)
		}
		return fmt.Sprintf("opened position for %s", p.Coin), client.OpenPosition(model.OpenPosition(p.Coin, t))
	}
}

// MultiStats allows the user to start and stop their own stats processors from the commands channel
// TODO : split responsibilities of this class to make things more clean and re-usable
func MultiStats(client api.TradeClient, user api.UserInterface) api.Processor {

	stats := &state{
		configs: make(map[time.Duration]windowConfig),
		windows: make(map[time.Duration]map[model.Coin]window),
	}

	// add default configs to start with ...
	for _, dd := range defaultDurations {
		log.Info().Int("min", int(dd.Minutes())).Msg("added default duration stats")
		stats.configs[dd] = newWindowConfig(dd)
		stats.windows[dd] = make(map[model.Coin]window)
	}

	//cmdSample := "?notify [time in minutes] [start/stop]"

	go trackUserActions(user, stats)

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {

		defer func() {
			log.Info().Str("processor", statsProcessorName).Msg("closing' strategy")
			close(out)
		}()

		for trade := range in {

			metrics.Observer.IncrementTrades(string(trade.Coin), statsProcessorName)

			for key, cfg := range stats.configs {

				// we got the config, check if we need something to do in the windows

				if _, ok := stats.windows[key][trade.Coin]; !ok {
					stats.windows[key][trade.Coin] = window{
						w: buffer.NewHistoryWindow(cfg.duration, int(cfg.historySizes)),
						c: buffer.NewMultiHMM(cfg.counterSizes...),
					}
					log.Info().
						Str("counters", fmt.Sprintf("%+v", cfg.counterSizes)).
						Int64("size", cfg.historySizes).
						Float64("duration", cfg.duration.Minutes()).
						Str("coin", string(trade.Coin)).
						Msg("started stats processor")
				}

				if _, ok := stats.windows[key][trade.Coin].w.Push(trade.Time, trade.Price, trade.Price*trade.Volume); ok {
					buckets := stats.windows[key][trade.Coin].w.Get(func(bucket interface{}) interface{} {
						// it's a history window , so we expect to have history buckets inside
						if b, ok := bucket.(buffer.TimeBucket); ok {
							// get the 'zerowth' stats element, as we are only assing the price a few lines above,
							// there is nothing more to retrieve from the bucket.
							priceView := buffer.NewView(b, 0)
							volumeView := buffer.NewView(b, 1)
							return windowView{
								price:  priceView,
								volume: volumeView,
							}
						}
						// TODO : this will break things a lot if we missed something ... 😅
						return nil
					})

					values, indicators, last := extractFromBuckets(buckets,
						order(func(b windowView) float64 {
							return b.price.Ratio
						}))

					// count the occurrences
					predictions := stats.windows[key][trade.Coin].c.Add(values[0][len(values[0])-1])

					// TODO : implement enrich on the model.Trade to pass data to downstream processors
					//trade.Enrich(MetaKey(trade.coin, int64(cfg.duration.Seconds())), buffer)
					metaKey := MetaKey(trade.Coin, key)
					aggregateStats := coinmath.NewAggregateStats(indicators)
					trade.Meta[MetaKeyIndicators(metaKey)] = aggregateStats
					trade.Meta[MetaStatsPredictionsKey(metaKey)] = predictions
					trade.Meta[MetaBucketKey(metaKey)] = last

					// TODO : send messages only if we are consuming live ...
					if user != nil && trade.Live {
						// TODO : add tests for this
						user.Send(api.Public,
							api.NewMessage(createStatsMessage(last, values, aggregateStats, predictions, trade, cfg)),
							api.NewTrigger(openPositionTrigger(trade, client)).
								WithID(uuid.New().String()).
								WithDescription("buy | sell"),
						)
					}
					// TODO : expose in metrics
					//fmt.Println(fmt.Sprintf("buffer = %+v", buffer))
				}
			}
			out <- trade
		}
		log.Info().Str("processor", statsProcessorName).Msg("closing processor")
	}
}

// MetaKey defines the meta key for this processor
func MetaKey(coin model.Coin, duration time.Duration) string {
	return fmt.Sprintf("%s|%.0f|%s", statsProcessorName, duration.Seconds(), string(coin))
}

// MetaRSIKey  returns the metadata key for the rsi stats.
func MetaKeyIndicators(metaKey string) string {
	return fmt.Sprintf("%s|%s", metaKey, "rsi")
}

// MetaIndicators returns the RSI stats from the metadata of the trade.
func MetaIndicators(trade *model.Trade, duration time.Duration) coinmath.AggregateStats {
	metaKey := MetaKey(trade.Coin, duration)
	if rsi, ok := trade.Meta[MetaKeyIndicators(metaKey)].(coinmath.AggregateStats); ok {
		return rsi
	}
	return coinmath.AggregateStats{}
}

// MetaBucketKey returns the metadata key for the last bucket stats.
func MetaBucketKey(metaKey string) string {
	return fmt.Sprintf("%s|%s", metaKey, "bucket")
}

// MetaBucket returns the last bucket stats from the metadata of the trade.
func MetaBucket(trade *model.Trade, duration time.Duration) windowView {
	metaKey := MetaKey(trade.Coin, duration)
	if bucketView, ok := trade.Meta[MetaBucketKey(metaKey)].(windowView); ok {
		return bucketView
	}
	return windowView{}
}

// MetaStatsPredictionsKey returns the metadata key for the stats predictions.
func MetaStatsPredictionsKey(metaKey string) string {
	return fmt.Sprintf("%s|%s", metaKey, "predictios")
}

// MetaStatsPredictions returns the statistical predictions from the trade metadata.
func MetaStatsPredictions(trade *model.Trade, duration time.Duration) map[string]buffer.Prediction {
	metaKey := MetaKey(trade.Coin, duration)
	if predictions, ok := trade.Meta[MetaStatsPredictionsKey(metaKey)].(map[string]buffer.Prediction); ok {
		return predictions
	}
	return map[string]buffer.Prediction{}
}

type windowView struct {
	price  buffer.TimeWindowView
	volume buffer.TimeWindowView
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

func order(extract func(b windowView) float64) func(b windowView) string {
	return func(b windowView) string {
		f := extract(b)
		v := coinmath.O10(f)
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

func createStatsMessage(last windowView, values [][]string, aggregateStats coinmath.AggregateStats, predictions map[string]buffer.Prediction, p *model.Trade, cfg windowConfig) string {
	// TODO : make the trigger arguments more specific to current stats state

	// format the predictions.
	pp := make([]string, len(predictions))
	i := 0
	for k, v := range predictions {
		keySlice := strings.Split(k, ":")
		valueSlice := strings.Split(v.Value, ":")
		emojiKeySlice := make([]string, len(keySlice))
		for j, ks := range keySlice {
			emojiKeySlice[j] = emoji.MapToSymbol(ks)
		}
		emojiValueSlice := make([]string, len(valueSlice))
		for k, vs := range valueSlice {
			emojiValueSlice[k] = emoji.MapToSymbol(vs)
		}
		pp[i] = fmt.Sprintf("%s <- %s ( %.2f : %.2f : %v ) ",
			strings.Join(emojiValueSlice, " "),
			strings.Join(emojiKeySlice, " "),
			v.Probability,
			1/float64(v.Options),
			v.Sample,
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

	// last bucket price details
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
		// predictions details
		strings.Join(pp, "\n "))

}
