package processor

import (
	"fmt"
	"reflect"
	"strconv"
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

// MultiStats allows the user to start and stop their own stats processors from the commands channel
func MultiStats(client api.TradeClient, user api.Interface, commands <-chan api.Command) model.Transform {

	windows := make(map[time.Duration]windowConfig)
	historyWindows := make(map[time.Duration]map[model.Coin]*buffer.HistoryWindow)

	//cmdSample := "?notify [time in minutes] [start/stop]"

	go func() {
		for command := range commands {
			if !strings.HasPrefix(command.Content, "?notify") {
				continue
			}
			cmd := strings.Split(command.Content, " ")[1:]
			if len(cmd) == 0 {
				user.SendWithRef("could not parse option args for '?notify'", command.ID, nil)
				continue
			}

			// second argument is the duration in minutes
			duration, err := strconv.ParseInt(cmd[0], 10, 64)
			if err != nil {
				user.SendWithRef(fmt.Sprintf("could not parse duration arg as int %v", cmd[0]), command.ID, nil)
				continue
			}
			timeDuration := time.Duration(duration) * time.Minute

			if len(cmd) < 2 {
				user.SendWithRef(fmt.Sprintf("a second arg for action is required [ start or stop ] %v", cmd), command.ID, nil)
				continue
			}

			action := cmd[1]
			switch action {
			case "start":
				if _, ok := windows[timeDuration]; ok {
					user.SendWithRef(fmt.Sprintf("notify window for '%v' mins is running ... please be patient", timeDuration.Minutes()), command.ID, nil)
					continue
				}
				if len(cmd) < 3 {
					user.SendWithRef(fmt.Sprintf("a third arg for size is required [ int ] %v", cmd), command.ID, nil)
					continue
				}
				size, err := strconv.ParseInt(cmd[2], 10, 64)
				if err != nil {
					user.SendWithRef(fmt.Sprintf("could not parse size arg as int %v", cmd[2]), command.ID, nil)
					continue
				}
				windows[timeDuration] = windowConfig{
					duration: timeDuration,
					size:     size,
				}
				historyWindows[timeDuration] = make(map[model.Coin]*buffer.HistoryWindow)
				user.SendWithRef(fmt.Sprintf("started notify window %v", cmd), command.ID, nil)
				continue
			case "stop":
				delete(windows, timeDuration)
				delete(historyWindows, timeDuration)
				user.SendWithRef(fmt.Sprintf("removed notify window for '%v' mins", timeDuration.Minutes()), command.ID, nil)
				continue
			default:
				user.SendWithRef(fmt.Sprintf("could not parse action arg as [ start or stop ] %v", cmd[1]), command.ID, nil)
				continue
			}
		}
	}()

	return func(in <-chan model.Trade, out chan<- model.Trade) error {

		defer func() {
			log.Info().Msg("closing 'Window' strategy")
			close(out)
		}()

		for p := range in {

			//metrics.Observe.Trades.WithLabelValues(p.Coin.String(), "multi_window").Inc()

			for key, cfg := range windows {

				// we got the config, check if we need something to do in the windows

				if _, ok := historyWindows[key][p.Coin]; !ok {
					historyWindows[key][p.Coin] = buffer.NewHistoryWindow(cfg.duration, int(cfg.size))
				}

				if _, ok := historyWindows[key][p.Coin].Push(p.Time.Real, p.Price); ok {

					buckets := historyWindows[key][p.Coin].Get(func(bucket interface{}) interface{} {
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
						api.SendMessage(user, uuid.New().String(), createStatsMessage(buckets, p, cfg), openPosition(p, client))
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
func Gap(percentage float64) model.Transform {
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
				fmt.Println(fmt.Sprintf("could not ifc[len(ifc)-1] = %+v", reflect.TypeOf(ifc[len(ifc)-1])))
			}
		} else {
			fmt.Println(fmt.Sprintf("could not parse = %+v", reflect.TypeOf(meta)))
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
	v := math.Order10(b.Ratio)
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
		t := model.NoType
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
