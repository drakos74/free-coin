package processor

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/math"

	"github.com/drakos74/free-coin/buffer"
	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/rs/zerolog/log"
)

const (
	RatioKey = "RATIO"
)

type windowConfig struct {
	duration time.Duration
	size     int64
}

type state struct {
	configs map[time.Duration]windowConfig
	windows map[time.Duration]map[model.Coin]*buffer.HistoryWindow
}

func trackUserActions(user api.UserInterface, stats *state) {
	for command := range user.Listen("stats", "?n") {
		// TODO :
		var duration int
		var action string
		var size int
		err := command.Validate(
			api.Any(),
			api.Contains("?n", "?notify"),
			api.Int(&duration),
			api.OneOf(&action, "start", "stop"),
			api.Int(&size))
		if err != nil {
			user.Reply(api.NewMessage(fmt.Sprintf("[error]: %s", err.Error())).ReplyTo(command.ID), err)
			continue
		}
		timeDuration := time.Duration(duration) * time.Minute

		switch action {
		case "start":
			if _, ok := stats.configs[timeDuration]; ok {
				user.Reply(
					api.NewMessage(fmt.Sprintf("notify window for '%v' mins is running ... please be patient", timeDuration.Minutes())).
						ReplyTo(command.ID), nil)
				continue
			}
			if size == 0 {
				user.Reply(
					api.NewMessage(fmt.Sprintf("a third arg for size is required [ int ] %v", size)).
						ReplyTo(command.ID), nil)
				continue
			}
			stats.configs[timeDuration] = windowConfig{
				duration: timeDuration,
				size:     int64(size),
			}
			stats.windows[timeDuration] = make(map[model.Coin]*buffer.HistoryWindow)
			user.Reply(
				api.NewMessage(fmt.Sprintf("started notify window %v", command.Content)).
					ReplyTo(command.ID), nil)
		case "stop":
			delete(stats.configs, timeDuration)
			delete(stats.windows, timeDuration)
			user.Reply(
				api.NewMessage(fmt.Sprintf("removed notify window for '%v' mins", timeDuration.Minutes())).
					ReplyTo(command.ID), nil)
		}
	}
}

// MultiStats allows the user to start and stop their own stats processors from the commands channel
func MultiStats(client api.TradeClient, user api.UserInterface) model.Processor {

	stats := &state{
		configs: make(map[time.Duration]windowConfig),
		windows: make(map[time.Duration]map[model.Coin]*buffer.HistoryWindow),
	}

	//cmdSample := "?notify [time in minutes] [start/stop]"

	go trackUserActions(user, stats)

	return func(in <-chan model.Trade, out chan<- model.Trade) error {

		defer func() {
			log.Info().Msg("closing 'Window' strategy")
			close(out)
		}()

		for p := range in {

			//metrics.Observe.Trades.WithLabelValues(p.Coin.String(), "multi_window").Inc()

			for key, cfg := range stats.configs {

				// we got the config, check if we need something to do in the windows

				if _, ok := stats.windows[key][p.Coin]; !ok {
					stats.windows[key][p.Coin] = buffer.NewHistoryWindow(cfg.duration, int(cfg.size))
				}

				if _, ok := stats.windows[key][p.Coin].Push(p.Time, p.Price); ok {

					buckets := stats.windows[key][p.Coin].Get(func(bucket interface{}) interface{} {
						// it's a history window , so we expect to have history buckets inside
						if b, ok := bucket.(buffer.TimeBucket); ok {
							return buffer.NewView(b, 0)
						}
						// TODO : this will break things a lot if we missed something ... 😅
						return nil
					})

					// TODO : implement enrich on the model.Trade to pass data to downstream processors
					//p.Enrich(MetaKey(p.Coin, int64(cfg.duration.Seconds())), buffer)
					if user != nil {
						// TODO : add tests for this
						user.Send(api.NewMessage(createStatsMessage(buckets, p, cfg)), api.NewTrigger(openPosition(p, client)).WithID(uuid.New().String()))
					}
					// TODO : expose in metrics
					//fmt.Println(fmt.Sprintf("buffer = %+v", buffer))
				}
			}
			out <- p
		}
		return nil
	}
}

// MetaKey defines the meta key for this trade
func MetaKey(coin model.Coin, duration int64) string {
	return fmt.Sprintf("%s:%s:%v", RatioKey, coin, duration)
}

// TODO :
// Gap calculates the time it takes for the price to move by the given percentage in any direction
func Gap(percentage float64) model.Processor {
	return func(in <-chan model.Trade, out chan<- model.Trade) error {
		return nil
	}
}

func ExtractLastBucket(coin model.Coin, key int64, trade *model.Trade) (buffer.TimeWindowView, bool) {
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
	symbol := emoji.DotSnow
	if b.Ratio > 0 {
		switch v {
		case 3:
			symbol = emoji.FirstEclipse
		case 2:
			symbol = emoji.FullMoon
		case 1:
			symbol = emoji.SunFace
		case 0:
			symbol = emoji.Star
		}
	} else {
		switch v {
		case 3:
			symbol = emoji.ThirdEclipse
		case 2:
			symbol = emoji.FullEclipse
		case 1:
			symbol = emoji.EclipseFace
		case 0:
			symbol = emoji.Comet
		}
	}
	return symbol
}

func createStatsMessage(buckets []interface{}, p model.Trade, cfg windowConfig) string {
	values, last := ExtractFromBuckets(buckets, order)

	// TODO : make the trigger arguments more specific to current stats state
	move := emoji.Zero
	if last.Ratio > 0 {
		move = emoji.Up
	} else if last.Ratio < 0 {
		move = emoji.Down
	}
	return fmt.Sprintf("%s|%.0fm: %s ... \n %s %s : %.2f : %.2f", p.Coin, cfg.duration.Minutes(), strings.Join(values, " "), move, math.Format(p.Price), last.Ratio*100, last.StdDev*100)

}

func openPosition(p model.Trade, client api.TradeClient) api.TriggerFunc {
	return func(command api.Command, options ...string) (string, error) {
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
