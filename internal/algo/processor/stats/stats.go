package stats

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/buffer"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
)

func trackUserActions(user api.User, stats *statsCollector) {
	for command := range user.Listen("stats", "?n") {
		var duration int
		var coin string
		var action string
		var strategy string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?n", "?notify"),
			api.Any(&coin),
			api.Int(&duration),
			api.Any(&strategy),
			api.OneOf(&action, "start", "stop", ""),
		)
		if err != nil {
			api.Reply(api.DrCoin, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}
		c := model.Coin(coin)
		d := time.Duration(duration) * time.Minute
		k := model.NewKey(c, d, strategy)
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
			//		api.Reply(api.DrCoin, user,
			//			api.NewMessage(fmt.Sprintf("notify window for '%v' mins is running ... please be patient", timeDuration.Minutes())).
			//				ReplyTo(command.ID), nil)
			//		continue
			//	} else {
			//		api.Reply(api.DrCoin, user,
			//			api.NewMessage(fmt.Sprintf("started notify window %v", command.Content)).
			//				ReplyTo(command.ID), nil)
			//	}
			//case "stop":
			//	stats.stop(timeDuration)
			//	api.Reply(api.DrCoin, user,
			//		api.NewMessage(fmt.Sprintf("removed notify window for '%v' mins", timeDuration.Minutes())).
			//			ReplyTo(command.ID), nil)
		}
	}
}

func sendWindowConfig(user api.User, k model.Key, w Window) {
	for _, cfg := range w.C.Config {
		api.Reply(api.DrCoin,
			user,
			api.NewMessage(fmt.Sprintf("%s|%v %v -> %v (%d)",
				k.Coin,
				k.Duration,
				cfg.LookBack,
				cfg.LookAhead,
				w.C.Status.Count),
			), nil)
	}
}

type TradeSignal struct {
	SignalEvent
	Predictions    map[buffer.Sequence]buffer.Predictions
	AggregateStats coinmath.AggregateStats
}

type SignalEvent struct {
	ID    string    `json:"id"`
	Key   model.Key `json:"key"`
	Price float64   `json:"price"`
	Time  time.Time `json:"time"`
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
