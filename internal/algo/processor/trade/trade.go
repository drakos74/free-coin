package trade

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor/stats"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const tradeProcessorName = "trade"

type trader struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	lock    sync.RWMutex
	configs map[time.Duration]map[model.Coin]OpenConfig
}

func newTrader() *trader {
	return &trader{
		lock:    sync.RWMutex{},
		configs: make(map[time.Duration]map[model.Coin]OpenConfig),
	}
}

func (tr *trader) init(dd time.Duration, coin model.Coin) {
	tr.lock.Lock()
	defer tr.lock.Unlock()
	if _, ok := tr.configs[dd]; !ok {
		tr.configs[dd] = make(map[model.Coin]OpenConfig)
	}
	if _, ok := tr.configs[dd][coin]; !ok {
		// take the config from btc
		if tmpCfg, ok := defaultOpenConfig[coin]; ok {
			tr.configs[dd][coin] = tmpCfg
			return
		}
		log.Error().Str("Coin", string(coin)).Msg("could not init config")
	}
}

func (tr *trader) get(dd time.Duration, coin model.Coin) OpenConfig {
	tr.lock.RLock()
	defer tr.lock.RUnlock()
	return tr.configs[dd][coin]
}

func (tr *trader) getAll() map[time.Duration][]OpenConfig {
	tr.lock.RLock()
	defer tr.lock.RUnlock()
	configs := make(map[time.Duration][]OpenConfig)
	for d, cfg := range tr.configs {
		cfgs := make([]OpenConfig, 0)
		for _, config := range cfg {
			cfgs = append(cfgs, config)
		}
		configs[d] = cfgs
	}
	return configs
}

func (tr *trader) set(dd time.Duration, coin model.Coin, probability float64, sample int) (time.Duration, OpenConfig) {
	tr.init(dd, coin)
	tr.lock.Lock()
	defer tr.lock.Unlock()
	cfg := tr.configs[dd][coin]
	cfg.ProbabilityThreshold = probability
	cfg.SampleThreshold = sample
	tr.configs[dd][coin] = cfg
	return dd, tr.configs[dd][coin]
}

func trackTraderActions(user api.User, trader *trader) {
	for command := range user.Listen("trader", "?t") {
		var duration int
		var coin string
		var probability float64
		var sample int
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?t", "?trade"),
			api.Any(&coin),
			api.Int(&duration),
			api.Float(&probability),
			api.Int(&sample),
		)
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}
		timeDuration := time.Duration(duration) * time.Minute

		c := model.Coin(coin)

		if probability > 0 {
			d, newConfig := trader.set(timeDuration, c, probability, sample)
			api.Reply(api.Private, user, api.NewMessage(fmt.Sprintf("%s %dm probability:%f sample:%d",
				newConfig.Coin,
				int(d.Minutes()),
				newConfig.ProbabilityThreshold,
				newConfig.SampleThreshold)), nil)
		} else {
			// return the current configs
			for d, config := range trader.getAll() {
				for _, cfg := range config {
					user.Send(api.Private,
						api.NewMessage(fmt.Sprintf("%s %dm probability:%f sample:%d",
							cfg.Coin,
							int(d.Minutes()),
							cfg.ProbabilityThreshold,
							cfg.SampleThreshold)), nil)
				}
			}
		}
	}
}

// Trade is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
// client is the exchange client used to open orders
// user is the user interface for interacting with the user
// block is the internal synchronisation mechanism used to make sure requests to the client are processed before proceeding
func Trade(client api.Exchange, user api.User, block api.Block) api.Processor {

	trader := newTrader()

	go trackTraderActions(user, trader)

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", tradeProcessorName).Msg("closing processor")
			close(out)
		}()

		for trade := range in {
			//metrics.Observer.IncrementTrades(string(trade.Coin), tradeProcessorName)
			// TODO : check also Active
			if trade == nil || !trade.Live || trade.Signal.Value == nil {
				out <- trade
				continue
			}

			if ts, ok := trade.Signal.Value.(stats.TradeSignal); ok {
				trader.init(ts.Duration, ts.Coin)
				// TODO : use an internal state like for the stats processor
				// we got a trade signal
				predictions := ts.Predictions
				if len(predictions) > 0 {
					cfg := trader.get(ts.Duration, ts.Coin)
					// check if we should make a buy order
					t := model.NoType
					pairs := make([]predictionPair, 0)
					for k, p := range predictions {
						if p.Probability >= cfg.ProbabilityThreshold && p.Sample >= cfg.SampleThreshold {
							// we know it s a good prediction. Lets check the value
							v := p.Value
							vv := strings.Split(v, ":")
							if t = cfg.contains(vv); t > 0 {
								pairs = append(pairs, predictionPair{
									t:           t,
									key:         k,
									value:       v,
									probability: p.Probability,
									base:        1 / float64(p.Options),
									sample:      p.Sample,
									label:       p.Label,
								})
							}
							log.Debug().
								Float64("probability", p.Probability).
								Int("sample", p.Sample).
								Strs("value", vv).
								Str("type", t.String()).
								Str("label", p.Label).
								Str("Coin", string(ts.Coin)).
								Msg("matched config")
						}
					}
					if t != model.NoType {
						if vol, ok := defaultOpenConfig[ts.Coin]; ok {
							order := model.NewOrder(ts.Coin).
								WithLeverage(model.L_5).
								WithVolume(vol.Volume).
								WithType(t).
								Market().
								Create()
							log.Info().
								Time("time", ts.Time).
								Str("ID", order.ID).
								Float64("price", ts.Price).
								Str("Coin", string(ts.Coin)).
								Msg("open position")
							err := client.OpenOrder(order)
							// notify other processes
							block.Action <- api.Action{}
							api.Reply(api.Private, user, api.NewMessage(createPredictionMessage(pairs)).AddLine(fmt.Sprintf("open %v %f %s at %f", t, vol.Volume, ts.Coin, ts.Price)), err)
							<-block.ReAction
						}
					}
				}
			}
			out <- trade
		}
	}
}

func createPredictionMessage(pairs []predictionPair) string {
	lines := make([]string, len(pairs))
	for i, pair := range pairs {
		kk := emoji.MapToSymbols(strings.Split(pair.key, ":"))
		vv := emoji.MapToSymbols(strings.Split(pair.value, ":"))
		pp := fmt.Sprintf("(%.2f | %.2f | %d)", pair.probability, pair.base, pair.sample)
		lines[i] = fmt.Sprintf("%s | %s -> %s %s", pair.label, strings.Join(kk, " "), strings.Join(vv, " "), pp)
	}
	return strings.Join(lines, "\n")
}

type predictionPair struct {
	label       string
	key         string
	value       string
	probability float64
	base        float64
	sample      int
	t           model.Type
}

var simpleStrategy = TradingStrategy{
	name: "simple",
	exec: func(vv []string) model.Type {
		t := model.NoType
		value := 0.0
		s := 0.0
		// add weight to the first one
		l := len(vv)
		if len(vv) == 1 {
			switch vv[0] {
			case "+1":
			case "+0":
				return model.Buy
			case "-1":
			case "-0":
				return model.Sell
			default:
				return model.NoType
			}
		}
		for w, v := range vv {
			it := toType(v)
			if t != model.NoType && it != t {
				return model.NoType
			}
			t = it
			i, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return t
			}
			g := float64(l-w) * i
			value += g
			s++
		}
		if math.Abs(value/s) <= 3 {
			return t
		}
		return model.NoType
	},
}

func toType(s string) model.Type {
	if strings.HasPrefix(s, "+") {
		return model.Buy
	}
	if strings.HasPrefix(s, "-") {
		return model.Sell
	}
	return model.NoType
}

var defaultOpenConfig = map[model.Coin]OpenConfig{
	model.BTC: {
		Coin:                 model.BTC,
		SampleThreshold:      5,
		ProbabilityThreshold: 0.51,
		Volume:               0.005,
		Strategies: []TradingStrategy{
			simpleStrategy,
		},
	},
	model.ETH: {
		Coin:                 model.ETH,
		SampleThreshold:      5,
		ProbabilityThreshold: 0.51,
		Volume:               0.15,
		Strategies: []TradingStrategy{
			simpleStrategy,
		},
	},
	model.LINK: {
		Coin:                 model.LINK,
		SampleThreshold:      5,
		ProbabilityThreshold: 0.51,
		Volume:               10,
		Strategies: []TradingStrategy{
			simpleStrategy,
		},
	},
	model.DOT: {
		Coin:                 model.DOT,
		SampleThreshold:      5,
		ProbabilityThreshold: 0.51,
		Volume:               10,
		Strategies: []TradingStrategy{
			simpleStrategy,
		},
	},
	model.XRP: {
		Coin:                 model.XRP,
		SampleThreshold:      5,
		ProbabilityThreshold: 0.51,
		Volume:               500,
		Strategies: []TradingStrategy{
			simpleStrategy,
		},
	},
}
