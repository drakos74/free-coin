package processor

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/drakos74/free-coin/coinapi"

	"github.com/google/uuid"

	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/math"

	"github.com/drakos74/free-coin/buffer"
	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/rs/zerolog/log"
)

const (
	RatioKey = "RATIO"
)

type windowConfig struct {
	duration     time.Duration
	historySizes int64
	counterSizes []int
}

func newWindowConfig(duration time.Duration) windowConfig {
	return windowConfig{
		duration:     duration,
		historySizes: 6,
		counterSizes: []int{2, 3, 4},
	}
}

type window struct {
	w *buffer.HistoryWindow
	c *buffer.Counter
}

type state struct {
	configs map[time.Duration]windowConfig
	windows map[time.Duration]map[coinapi.Coin]window
}

func trackUserActions(user model.UserInterface, stats *state) {
	for command := range user.Listen("stats", "?n") {
		// TODO :
		var duration int
		var action string
		err := command.Validate(
			coinapi.Any(),
			coinapi.Contains("?n", "?notify"),
			coinapi.Int(&duration),
			coinapi.OneOf(&action, "start", "stop"),
		)
		if err != nil {
			model.Reply(user,
				coinapi.
					NewMessage(fmt.Sprintf("[error]: %s", err.Error())).
					ReplyTo(command.ID), err)
			continue
		}
		timeDuration := time.Duration(duration) * time.Minute

		switch action {
		case "start":
			if _, ok := stats.configs[timeDuration]; ok {
				model.Reply(user,
					coinapi.
						NewMessage(fmt.Sprintf("notify window for '%v' mins is running ... please be patient", timeDuration.Minutes())).
						ReplyTo(command.ID), nil)
				continue
			}
			// TODO : decide how to handle the historySizes, especially in combination with the counterSizes.
			stats.configs[timeDuration] = newWindowConfig(timeDuration)
			stats.windows[timeDuration] = make(map[coinapi.Coin]window)
			model.Reply(user,
				coinapi.
					NewMessage(fmt.Sprintf("started notify window %v", command.Content)).
					ReplyTo(command.ID), nil)
		case "stop":
			delete(stats.configs, timeDuration)
			delete(stats.windows, timeDuration)
			model.Reply(user,
				coinapi.
					NewMessage(fmt.Sprintf("removed notify window for '%v' mins", timeDuration.Minutes())).
					ReplyTo(command.ID), nil)
		}
	}
}

func openPositionTrigger(p coinapi.Trade, client model.TradeClient) coinapi.TriggerFunc {
	return func(command coinapi.Command, options ...string) (string, error) {
		var t coinapi.Type
		switch command.Content {
		case "buy":
			t = coinapi.Buy
		case "sell":
			t = coinapi.Sell
		default:
			return "[error]", fmt.Errorf("unknown command: %s", command.Content)
		}
		return fmt.Sprintf("opened position for %s", p.Coin), client.OpenPosition(model.OpenPosition(p.Coin, t))
	}
}

// MultiStats allows the user to start and stop their own stats processors from the commands channel
func MultiStats(client model.TradeClient, user model.UserInterface) coinapi.Processor {

	stats := &state{
		configs: make(map[time.Duration]windowConfig),
		windows: make(map[time.Duration]map[coinapi.Coin]window),
	}

	//cmdSample := "?notify [time in minutes] [start/stop]"

	go trackUserActions(user, stats)

	return func(in <-chan coinapi.Trade, out chan<- coinapi.Trade) {

		defer func() {
			log.Info().Msg("closing 'Window' strategy")
			close(out)
		}()

		for p := range in {

			//metrics.Observe.Trades.WithLabelValues(p.Coin.String(), "multi_window").Inc()

			for key, cfg := range stats.configs {

				// we got the config, check if we need something to do in the windows

				if _, ok := stats.windows[key][p.Coin]; !ok {
					stats.windows[key][p.Coin] = window{
						w: buffer.NewHistoryWindow(cfg.duration, int(cfg.historySizes)),
						c: buffer.NewCounter(2, 3, 4),
					}
				}

				if _, ok := stats.windows[key][p.Coin].w.Push(p.Time, p.Price); ok {
					buckets := stats.windows[key][p.Coin].w.Get(func(bucket interface{}) interface{} {
						// it's a history window , so we expect to have history buckets inside
						if b, ok := bucket.(buffer.TimeBucket); ok {
							// get the 'zerowth' stats element, as we are only assing the price a few lines above,
							// there is nothing more to retrieve from the bucket.
							return buffer.NewView(b, 0)
						}
						// TODO : this will break things a lot if we missed something ... 😅
						return nil
					})

					values, last := ExtractFromBuckets(buckets, order)

					// count the occurrences
					predictions := stats.windows[key][p.Coin].c.Add(values[len(values)-1])

					// TODO : implement enrich on the model.Trade to pass data to downstream processors
					//p.Enrich(MetaKey(p.Coin, int64(cfg.duration.Seconds())), buffer)
					if user != nil {
						// TODO : add tests for this
						user.Send(coinapi.NewMessage(createStatsMessage(last, values, predictions, p, cfg)), coinapi.NewTrigger(openPositionTrigger(p, client)).WithID(uuid.New().String()))
					}
					// TODO : expose in metrics
					//fmt.Println(fmt.Sprintf("buffer = %+v", buffer))
				}
			}
			out <- p
		}
	}
}

// MetaKey defines the meta key for this trade
func MetaKey(coin coinapi.Coin, duration int64) string {
	return fmt.Sprintf("%s:%s:%v", RatioKey, coin, duration)
}

// TODO :
// Gap calculates the time it takes for the price to move by the given percentage in any direction
func Gap(percentage float64) coinapi.Processor {
	return func(in <-chan coinapi.Trade, out chan<- coinapi.Trade) {}
}

func ExtractLastBucket(coin coinapi.Coin, key int64, trade *coinapi.Trade) (buffer.TimeWindowView, bool) {
	k := MetaKey(coin, key)
	if meta, ok := trade.Meta[k]; ok {
		if ifc, ok := meta.([]interface{}); ok {
			if bucket, ok := ifc[len(ifc)-1].(buffer.TimeWindowView); ok {
				return bucket, true
			} else {
				log.Warn().Str("type", fmt.Sprintf("%+v", reflect.TypeOf(ifc[len(ifc)-1]))).Msg("could not get expected type")
			}
		} else {
			log.Warn().Str("key", fmt.Sprintf("%+v", k)).Msg("key not found in trade metadata")
		}
	}
	return buffer.TimeWindowView{}, false
}

func ExtractFromBuckets(ifc interface{}, format func(b buffer.TimeWindowView) string) ([]string, buffer.TimeWindowView) {
	s := reflect.ValueOf(ifc)
	bb := make([]string, s.Len())
	var last buffer.TimeWindowView
	for i := 0; i < s.Len(); i++ {
		b := s.Index(i).Interface().(buffer.TimeWindowView)
		last = b
		bb[i] = format(b)
	}
	return bb, last
}

func order(b buffer.TimeWindowView) string {
	v := math.O10(b.Ratio)

	fs := "%d"
	if b.Ratio > 0 {
		fs = fmt.Sprintf("+%s", fs)
	} else if b.Ratio < 0 {
		fs = fmt.Sprintf("-%s", fs)
	}

	s := fmt.Sprintf("+%d", v)
	return s
}

func createStatsMessage(last buffer.TimeWindowView, values []string, predictions map[string]buffer.Prediction, p coinapi.Trade, cfg windowConfig) string {

	// TODO : make the trigger arguments more specific to current stats state
	// identify the move of the coin.
	move := emoji.Zero
	if last.Ratio > 0 {
		move = emoji.Up
	} else if last.Ratio < 0 {
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
	emojiValues := make([]string, len(values))
	for j := 0; j < len(values)-1; j++ {
		emojiValues[j] = emoji.MapToSymbol(values[j])
	}

	// format the status message for the processor.
	return fmt.Sprintf("%s|%.0fm: %s ... \n %s %s : %.2f : %.2f\n%s",
		p.Coin,
		cfg.duration.Minutes(),
		strings.Join(emojiValues, " "),
		move,
		math.Format(p.Price),
		last.Ratio*100,
		last.StdDev*100,
		strings.Join(pp, "\n"),
	)

}
