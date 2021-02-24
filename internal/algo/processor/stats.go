package processor

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/emoji"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	statsProcessorName = "stats"
)

var (
	defaultDurations = []time.Duration{10 * time.Minute, 30 * time.Minute, 2 * time.Hour}
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

type statsCollector struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	lock    sync.RWMutex
	configs map[time.Duration]windowConfig
	windows map[time.Duration]map[model.Coin]window
}

func newStats(config []time.Duration) *statsCollector {
	stats := &statsCollector{
		lock:    sync.RWMutex{},
		configs: make(map[time.Duration]windowConfig),
		windows: make(map[time.Duration]map[model.Coin]window),
	}
	// add default configs to start with ...
	for _, dd := range config {
		log.Info().Int("min", int(dd.Minutes())).Msg("added default duration stats")
		stats.configs[dd] = newWindowConfig(dd)
		stats.windows[dd] = make(map[model.Coin]window)
	}
	return stats
}

func (s *statsCollector) hasOrAddDuration(dd time.Duration) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if _, ok := s.configs[dd]; ok {
		return true
	}
	s.configs[dd] = newWindowConfig(dd)
	return false
}

func (s *statsCollector) start(dd time.Duration, coin model.Coin) {
	s.lock.Lock()
	defer s.lock.Unlock()
	cfg := s.configs[dd]
	if _, ok := s.windows[dd][coin]; !ok {
		s.windows[dd][coin] = window{
			w: buffer.NewHistoryWindow(cfg.duration, int(cfg.historySizes)),
			c: buffer.NewMultiHMM(cfg.counterSizes...),
		}
		log.Info().
			Str("counters", fmt.Sprintf("%+v", cfg.counterSizes)).
			Int64("size", cfg.historySizes).
			Float64("duration", cfg.duration.Minutes()).
			Str("coin", string(coin)).
			Msg("started stats processor")
	}
}

func (s *statsCollector) stop(dd time.Duration) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.configs, dd)
	delete(s.windows, dd)
}

func (s *statsCollector) push(dd time.Duration, trade *model.Trade) ([]interface{}, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.windows[dd][trade.Coin].w.Push(trade.Time, trade.Price, trade.Price*trade.Volume); ok {
		buckets := s.windows[dd][trade.Coin].w.Get(func(bucket interface{}) interface{} {
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
		return buckets, ok
	}
	return nil, false
}

func (s *statsCollector) add(dd time.Duration, coin model.Coin, v string) (map[string]buffer.Prediction, buffer.Status) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.windows[dd][coin].c.Add(v, fmt.Sprintf("%dm", int(dd.Minutes())))
}

func trackStatsActions(user api.User, stats *statsCollector) {
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
			// TODO : decide how to handle the historySizes, especially in combination with the counterSizes.
			if stats.hasOrAddDuration(timeDuration) {
				api.Reply(api.Private, user,
					api.NewMessage(fmt.Sprintf("notify window for '%v' mins is running ... please be patient", timeDuration.Minutes())).
						ReplyTo(command.ID), nil)
				continue
			} else {
				api.Reply(api.Private, user,
					api.NewMessage(fmt.Sprintf("started notify window %v", command.Content)).
						ReplyTo(command.ID), nil)
			}
		case "stop":
			stats.stop(timeDuration)
			api.Reply(api.Private, user,
				api.NewMessage(fmt.Sprintf("removed notify window for '%v' mins", timeDuration.Minutes())).
					ReplyTo(command.ID), nil)
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
//			order := model.NewOrder(p.Coin).
//				WithLeverage(model.L_5).
//				WithVolume(vol.volume).
//				WithType(t).
//				Market().
//				Create()
//			return fmt.Sprintf("opened position for %s", p.Coin), client.OpenOrder(order)
//
//		} else {
//			return "could not open position", fmt.Errorf("no pre-defined volume for %s", p.Coin)
//		}
//	}
//}

// MultiStats allows the user to start and stop their own stats processors from the commands channel
// TODO : split responsibilities of this class to make things more clean and re-usable
func MultiStats(client api.Exchange, user api.User, signal chan<- api.Signal) api.Processor {

	stats := newStats(defaultDurations)

	//cmdSample := "?notify [time in minutes] [start/stop]"

	go trackStatsActions(user, stats)

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {

		defer func() {
			log.Info().Str("processor", statsProcessorName).Msg("closing' strategy")
			close(out)
		}()

		for trade := range in {

			metrics.Observer.IncrementTrades(string(trade.Coin), statsProcessorName)

			for key, cfg := range stats.configs {
				// we got the config, check if we need something to do in the windows
				stats.start(key, trade.Coin)

				// push the trade data to the stats collector window
				if buckets, ok := stats.push(key, trade); ok {
					values, indicators, last := extractFromBuckets(buckets,
						order(func(b windowView) float64 {
							return b.price.Ratio
						}))
					// count the occurrences
					predictions, status := stats.add(key, trade.Coin, values[0][len(values[0])-1])
					if trade.Live {
						aggregateStats := coinmath.NewAggregateStats(indicators)
						signal <- api.Signal{
							Type: "tradeSignal",
							Value: tradeSignal{
								coin:           trade.Coin,
								price:          trade.Price,
								time:           trade.Time,
								duration:       key,
								predictions:    predictions,
								aggregateStats: aggregateStats,
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
		log.Info().Str("processor", statsProcessorName).Msg("closing processor")
	}
}

type tradeSignal struct {
	coin           model.Coin
	price          float64
	time           time.Time
	duration       time.Duration
	predictions    map[string]buffer.Prediction
	aggregateStats coinmath.AggregateStats
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

func createStatsMessage(last windowView, values [][]string, aggregateStats coinmath.AggregateStats, predictions map[string]buffer.Prediction, status buffer.Status, p *model.Trade, cfg windowConfig) string {
	// TODO : make the trigger arguments more specific to current stats statsCollector

	// format the predictions.
	pp := make([]string, len(predictions)+1)
	samples := make([]string, len(status.Samples))
	k := 0
	for cfg, sample := range status.Samples {
		samples[k] = fmt.Sprintf("%v : %d", cfg, len(sample))
		k++
	}
	pp[0] = fmt.Sprintf("%+v -> %s", status.Count, strings.Join(samples, " | "))
	i := 1
	for k, v := range predictions {
		keySlice := strings.Split(k, ":")
		valueSlice := strings.Split(v.Value, ":")
		emojiKeySlice := emoji.MapToSymbols(keySlice)
		emojiValueSlice := emoji.MapToSymbols(valueSlice)
		pp[i] = fmt.Sprintf("%s -> %s ( %.2f : %.2f : %v : %d : %d) ",
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
