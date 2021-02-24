package processor

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const tradeProcessorName = "trade"

type trader struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	lock    sync.RWMutex
	configs map[time.Duration]map[model.Coin]openConfig
}

func newTrader() *trader {
	return &trader{
		lock:    sync.RWMutex{},
		configs: make(map[time.Duration]map[model.Coin]openConfig),
	}
}

func (tr *trader) init(dd time.Duration, coin model.Coin) {
	tr.lock.Lock()
	defer tr.lock.Unlock()
	if _, ok := tr.configs[dd]; !ok {
		tr.configs[dd] = make(map[model.Coin]openConfig)
	}
	if _, ok := tr.configs[dd][coin]; !ok {
		// take the config from btc
		if tmpCfg, ok := defaultOpenConfig[coin]; ok {
			tr.configs[dd][coin] = tmpCfg
			return
		}
		log.Error().Str("coin", string(coin)).Msg("could not init config")
	}
}

func (tr *trader) get(dd time.Duration, coin model.Coin) openConfig {
	tr.lock.RLock()
	defer tr.lock.RUnlock()
	return tr.configs[dd][coin]
}

func (tr *trader) getAll() map[time.Duration][]openConfig {
	tr.lock.RLock()
	defer tr.lock.RUnlock()
	configs := make(map[time.Duration][]openConfig)
	for d, cfg := range tr.configs {
		cfgs := make([]openConfig, 0)
		for _, config := range cfg {
			cfgs = append(cfgs, config)
		}
		configs[d] = cfgs
	}
	return configs
}

func (tr *trader) set(dd time.Duration, coin model.Coin, probability float64, sample int) (time.Duration, openConfig) {
	tr.init(dd, coin)
	tr.lock.Lock()
	defer tr.lock.Unlock()
	cfg := tr.configs[dd][coin]
	cfg.probabilityThreshold = probability
	cfg.sampleThreshold = sample
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
				newConfig.coin,
				int(d.Minutes()),
				newConfig.probabilityThreshold,
				newConfig.sampleThreshold)), nil)
		} else {
			// return the current configs
			for d, config := range trader.getAll() {
				for _, cfg := range config {
					user.Send(api.Private,
						api.NewMessage(fmt.Sprintf("%s %dm probability:%f sample:%d",
							cfg.coin,
							int(d.Minutes()),
							cfg.probabilityThreshold,
							cfg.sampleThreshold)), nil)
				}
			}
		}
	}
}

// Trade is the processor responsible for making trade decisions.
// this processor should analyse the triggers from previous processors and ...
// open positions, track and close appropriately.
func Trade(client api.Exchange, user api.User, action chan<- api.Action, signal <-chan api.Signal) api.Processor {

	trader := newTrader()

	go trackTraderActions(user, trader)

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", tradeProcessorName).Msg("closing' strategy")
			close(out)
		}()

		for {
			select {
			case trade := <-in:
				metrics.Observer.IncrementTrades(string(trade.Coin), tradeProcessorName)
				// TODO : check also Active
				if !trade.Live {
					out <- trade
					continue
				}
				out <- trade
			case s := <-signal:
				if ts, ok := s.Value.(tradeSignal); ok {
					trader.init(ts.duration, ts.coin)
					// TODO : use an internal state like for the stats processor
					// we got a trade signal
					predictions := ts.predictions
					if len(predictions) > 0 {
						cfg := trader.get(ts.duration, ts.coin)
						// check if we should make a buy order
						t := model.NoType
						pairs := make([]predictionPair, 0)
						for k, p := range predictions {
							if p.Probability >= cfg.probabilityThreshold && p.Sample >= cfg.sampleThreshold {
								// we know it s a good prediction. Lets check the value
								v := p.Value
								vv := strings.Split(v, ":")
								if t = cfg.contains(vv); t > 0 {
									pairs = append(pairs, predictionPair{
										t:     t,
										key:   k,
										value: v,
										label: p.Label,
									})
								}
								log.Info().
									Float64("probability", p.Probability).
									Int("sample", p.Sample).
									Strs("value", vv).
									Str("type", t.String()).
									Str("label", p.Label).
									Str("coin", string(ts.coin)).
									Msg("matched config")
							}
						}
						if t != model.NoType {
							if vol, ok := defaultOpenConfig[ts.coin]; ok {
								log.Info().
									Time("time", ts.time).
									Str("coin", string(ts.coin)).
									Msg("open order")
								// TODO : print the prediction in the reply message
								err := client.OpenOrder(model.NewOrder(ts.coin).
									WithLeverage(model.L_5).
									WithVolume(vol.volume).
									WithType(t).
									Market().
									Create())
								// notify other processes
								action <- api.Action{}
								// TODO : combine with the trades to know of the price
								api.Reply(api.Private, user, api.NewMessage(createPredictionMessage(pairs)).AddLine(fmt.Sprintf("open %v %f %s at %f", t, vol.volume, ts.coin, ts.price)), err)
							}
						}
					}
				} else {
					log.Warn().
						Str("type", s.Type).
						Str("value", reflect.TypeOf(s.Value).Name()).
						Msg("could not parse signal")
				}
			}
		}
		// TODO : what happens when finished ... how do we close the processor
		log.Info().Str("processor", tradeProcessorName).Msg("closing processor")
	}
}

func createPredictionMessage(pairs []predictionPair) string {
	lines := make([]string, len(pairs))
	for i, pair := range pairs {
		kk := emoji.MapToSymbols(strings.Split(pair.key, ":"))
		vv := emoji.MapToSymbols(strings.Split(pair.value, ":"))
		lines[i] = fmt.Sprintf("%s | %s -> %s", pair.label, strings.Join(kk, " "), strings.Join(vv, " "))
	}
	return strings.Join(lines, "\n")
}

type predictionPair struct {
	label string
	key   string
	value string
	t     model.Type
}

type openConfig struct {
	coin                 model.Coin
	sampleThreshold      int
	probabilityThreshold float64
	volume               float64
	strategies           []tradingStrategy
}

type tradingStrategy struct {
	name string
	exec func(vv []string) model.Type
}

func newOpenConfig(c model.Coin, vol float64) openConfig {
	return openConfig{
		coin:   c,
		volume: vol,
		strategies: []tradingStrategy{
			simpleStrategy,
		},
	}
}
func (c openConfig) withProbability(p float64) openConfig {
	c.probabilityThreshold = p
	return c
}

func (c openConfig) withSample(s int) openConfig {
	c.sampleThreshold = s
	return c
}

func (c openConfig) contains(vv []string) model.Type {
	for _, strategy := range c.strategies {
		if t := strategy.exec(vv); t != model.NoType {
			return t
		}
	}
	return model.NoType
}

var simpleStrategy = tradingStrategy{
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
			case "+2":
				return model.Buy
			case "-1":
			case "-0":
			case "-2":
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

var defaultOpenConfig = map[model.Coin]openConfig{
	model.BTC: {
		coin:                 model.BTC,
		sampleThreshold:      3,
		probabilityThreshold: 0.45,
		volume:               0.01,
		strategies: []tradingStrategy{
			simpleStrategy,
		},
	},
	model.ETH: {
		coin:                 model.ETH,
		sampleThreshold:      3,
		probabilityThreshold: 0.45,
		volume:               0.3,
		strategies: []tradingStrategy{
			simpleStrategy,
		},
	},
	model.LINK: {
		coin:                 model.LINK,
		sampleThreshold:      3,
		probabilityThreshold: 0.45,
		volume:               15,
		strategies: []tradingStrategy{
			simpleStrategy,
		},
	},
	model.DOT: {
		coin:                 model.DOT,
		sampleThreshold:      3,
		probabilityThreshold: 0.45,
		strategies: []tradingStrategy{
			simpleStrategy,
		},
	},
	model.XRP: {
		coin:                 model.XRP,
		sampleThreshold:      3,
		probabilityThreshold: 0.45,
		volume:               1000,
		strategies: []tradingStrategy{
			simpleStrategy,
		},
	},
}
