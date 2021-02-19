package processor

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/metrics"

	"github.com/drakos74/free-coin/internal/api"

	"github.com/google/uuid"

	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/math"

	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/rs/zerolog/log"
)

const (
	statsProcessorName = "stats"
)

var (
	defaultDurations = []time.Duration{10 * time.Minute, 30 * time.Minute}
)

type windowConfig struct {
	duration     time.Duration
	historySizes int64
	counterSizes []int
}

func newWindowConfig(duration time.Duration) windowConfig {
	return windowConfig{
		duration:     duration,
		historySizes: 14,
		counterSizes: []int{3, 4, 5},
	}
}

type window struct {
	w *buffer.HistoryWindow
	c *buffer.HMM
}

type state struct {
	configs map[time.Duration]windowConfig
	windows map[time.Duration]map[api.Coin]window
}

func trackUserActions(user model.UserInterface, stats *state) {
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
			model.Reply(user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}
		timeDuration := time.Duration(duration) * time.Minute

		switch action {
		case "":
			// TODO : return the currently running stats processes
		case "start":
			if _, ok := stats.configs[timeDuration]; ok {
				model.Reply(user,
					api.NewMessage(fmt.Sprintf("notify window for '%v' mins is running ... please be patient", timeDuration.Minutes())).
						ReplyTo(command.ID), nil)
				continue
			}
			// TODO : decide how to handle the historySizes, especially in combination with the counterSizes.
			stats.configs[timeDuration] = newWindowConfig(timeDuration)
			stats.windows[timeDuration] = make(map[api.Coin]window)
			model.Reply(user,
				api.NewMessage(fmt.Sprintf("started notify window %v", command.Content)).
					ReplyTo(command.ID), nil)
		case "stop":
			delete(stats.configs, timeDuration)
			delete(stats.windows, timeDuration)
			model.Reply(user,
				api.NewMessage(fmt.Sprintf("removed notify window for '%v' mins", timeDuration.Minutes())).
					ReplyTo(command.ID), nil)
		}
	}
}

func openPositionTrigger(p *api.Trade, client model.TradeClient) api.TriggerFunc {
	return func(command api.Command) (string, error) {
		var t api.Type
		switch command.Content {
		case "buy":
			t = api.Buy
		case "sell":
			t = api.Sell
		default:
			return "[error]", fmt.Errorf("unknown command: %s", command.Content)
		}
		return fmt.Sprintf("opened position for %s", p.Coin), client.OpenPosition(model.OpenPosition(p.Coin, t))
	}
}

// MultiStats allows the user to start and stop their own stats processors from the commands channel
// TODO : split responsibilities of this class to make things more clean and re-usable
func MultiStats(client model.TradeClient, user model.UserInterface) api.Processor {

	stats := &state{
		configs: make(map[time.Duration]windowConfig),
		windows: make(map[time.Duration]map[api.Coin]window),
	}

	// add default configs to start with ...
	for _, dd := range defaultDurations {
		log.Info().Int("min", int(dd.Minutes())).Msg("added default duration stats")
		stats.configs[dd] = newWindowConfig(dd)
		stats.windows[dd] = make(map[api.Coin]window)
	}

	//cmdSample := "?notify [time in minutes] [start/stop]"

	go trackUserActions(user, stats)

	return func(in <-chan *api.Trade, out chan<- *api.Trade) {

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
						Ints("counters", cfg.counterSizes).
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

					values, rsi, ema, last := extractFromBuckets(buckets,
						order(func(b windowView) float64 {
							return b.price.Ratio
						}))

					// count the occurrences
					predictions := stats.windows[key][trade.Coin].c.Add(values[0][len(values[0])-1])

					// TODO : implement enrich on the model.Trade to pass data to downstream processors
					//trade.Enrich(MetaKey(trade.coin, int64(cfg.duration.Seconds())), buffer)
					metaKey := MetaKey(trade.Coin, key)
					trade.Meta[MetaKeyRSI(metaKey)] = AggregateStats{
						RSI:    rsi,
						EMA:    ema,
						Sample: len(values),
					}
					trade.Meta[MetaStatsPredictionsKey(metaKey)] = predictions
					trade.Meta[MetaBucketKey(metaKey)] = last

					// TODO : send messages only if we are consuming live ...
					if user != nil && trade.Live {
						// TODO : add tests for this
						user.Send(
							api.NewMessage(createStatsMessage(last, values, rsi, ema, predictions, trade, cfg)),
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
func MetaKey(coin api.Coin, duration time.Duration) string {
	return fmt.Sprintf("%s|%.0f|%s", statsProcessorName, duration.Seconds(), string(coin))
}

// MetaRSIKey  returns the metadata key for the rsi stats.
func MetaKeyRSI(metaKey string) string {
	return fmt.Sprintf("%s|%s", metaKey, "rsi")
}

// MetaRSI returns the RSI stats from the metadata of the trade.
func MetaRSI(trade *api.Trade, duration time.Duration) AggregateStats {
	metaKey := MetaKey(trade.Coin, duration)
	if rsi, ok := trade.Meta[MetaKeyRSI(metaKey)].(AggregateStats); ok {
		return rsi
	}
	return AggregateStats{}
}

// MetaBucketKey returns the metadata key for the last bucket stats.
func MetaBucketKey(metaKey string) string {
	return fmt.Sprintf("%s|%s", metaKey, "bucket")
}

// MetaBucket returns the last bucket stats from the metadata of the trade.
func MetaBucket(trade *api.Trade, duration time.Duration) windowView {
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
func MetaStatsPredictions(trade *api.Trade, duration time.Duration) map[string]buffer.Prediction {
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
func extractFromBuckets(ifc interface{}, format ...func(b windowView) string) ([][]string, int, float64, windowView) {
	s := reflect.ValueOf(ifc)
	pp := make([][]string, s.Len())
	var last windowView
	rsiStream := &math.RSI{}
	var rsi int
	var ema float64
	l := s.Len()
	for i := 0; i < l; i++ {
		b := s.Index(i).Interface().(windowView)
		last = b
		pp[i] = make([]string, len(format))
		for j, f := range format {
			pp[i][j] = f(b)
		}
		rsi, _ = rsiStream.Add(b.price.Diff)
		w := 2 / float64(l)
		ema = b.price.Value*w + ema*(1-w)
	}
	return pp, rsi, ema, last
}

func order(extract func(b windowView) float64) func(b windowView) string {
	return func(b windowView) string {
		f := extract(b)
		v := math.O10(f)
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

func createStatsMessage(last windowView, values [][]string, rsi int, ema float64, predictions map[string]buffer.Prediction, p *api.Trade, cfg windowConfig) string {
	// TODO : make the trigger arguments more specific to current stats state
	// identify the move of the coin.
	move := emoji.Zero
	if last.price.Ratio > 0 {
		move = emoji.Up
	} else if last.price.Ratio < 0 {
		move = emoji.Down
	}

	// format the predictions.
	pp := make([]string, len(predictions))
	i := 0
	for k, v := range predictions {
		valueSlice := strings.Split(k, ":")
		emojiSlice := make([]string, len(valueSlice))
		for j, vs := range valueSlice {
			emojiSlice[j] = emoji.MapToSymbol(vs)
		}
		pp[i] = fmt.Sprintf("%s <- %s ( %.2f : %.2f : %v ) ",
			emoji.MapToSymbol(v.Value),
			strings.Join(emojiSlice, " "),
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
	mp := fmt.Sprintf("%s %s ratio:%.2f stdv:%.2f ema:%.2f",
		move,
		math.Format(p.Price),
		last.price.Ratio*100,
		last.price.StdDev,
		last.price.EMADiff)

	mv := fmt.Sprintf("%f %s ratio:%.2f stdv:%.2f ema:%.2f",
		last.volume.Diff,
		math.Format(last.volume.Value),
		last.volume.Ratio*100,
		last.volume.StdDev,
		last.price.EMADiff)

	// bucket collector details
	st := fmt.Sprintf("rsi:%d ema:%.2f (%d)",
		rsi,
		100*(ema-last.price.Value)/last.price.Value,
		len(values))

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

// AggregateStats are the aggregate stats of the bucket windows.
type AggregateStats struct {
	RSI    int
	EMA    float64
	Sample int
}
