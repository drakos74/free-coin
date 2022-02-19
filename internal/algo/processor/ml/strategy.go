package ml

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

type config struct {
	bufferTime     float64
	priceThreshold float64
	weight         int
	enabled        bool
}

// strategy is responsible for making a call on weather the signal is good enough to trade on.
type strategy struct {
	lock    *sync.RWMutex
	signals map[model.Key]Signal
	trades  map[model.Key]model.Trade
	config  map[model.Key]config
	enabled map[model.Coin]bool
	live    map[model.Coin]bool
}

func newStrategy(segments map[model.Key]Segments) *strategy {
	cfg := make(map[model.Key]config)
	enabled := make(map[model.Coin]bool)
	for k, segment := range segments {
		cfg[k] = config{
			bufferTime:     segment.Trader.BufferTime,
			priceThreshold: segment.Trader.PriceThreshold,
			weight:         segment.Trader.Weight,
			enabled:        true,
		}
		// auto-enable
		enabled[k.Coin] = true
		log.Info().
			Str("key", fmt.Sprintf("%+v", k)).
			Str("config", fmt.Sprintf("%+v", cfg[k])).
			Msg("init strategy")
	}
	return &strategy{
		lock:    new(sync.RWMutex),
		signals: make(map[model.Key]Signal),
		trades:  make(map[model.Key]model.Trade),
		config:  cfg,
		live:    make(map[model.Coin]bool),
		enabled: enabled,
	}
}

func (str *strategy) enable(c model.Coin, enabled bool) []bool {
	str.lock.Lock()
	defer str.lock.Unlock()
	for coin, _ := range str.enabled {
		if c == model.AllCoins || c == coin {
			str.enabled[coin] = enabled
		}
	}
	ee := make([]bool, 0)
	for coin, ok := range str.enabled {
		if c == model.AllCoins || c == coin {
			ee = append(ee, ok)
		}
	}
	return ee
}

func (str *strategy) isLive(trade *model.Trade) (bool, bool) {
	if str.live[trade.Coin] {
		return true, false
	}
	if cointime.IsValidTime(trade.Time) {
		log.Info().Time("stamp", trade.Time).Str("coin", string(trade.Coin)).Msg("strategy going live")
		str.live[trade.Coin] = true
		return true, true
	}
	return false, false
}

func (str *strategy) eval(trade *model.Trade, signals map[time.Duration]Signal, config *Config) (Signal, model.Key, bool, bool) {
	// strategy is not live yet ... trade is past one
	if live, _ := str.isLive(trade); !live && !config.Option.Debug {
		return Signal{}, model.Key{}, false, false
	}
	if len(signals) > 0 {
		// TODO : decide how to make a unified trading strategy for the real trading
		var signal Signal
		var act = true
		strategyBuffer := new(strings.Builder)
		trend := make(map[model.Type][]time.Duration)
		var price float64
		var precision float64
		for _, s := range signals {
			if _, ok := trend[s.Type]; !ok {
				trend[s.Type] = make([]time.Duration, 0)
			}
			trend[s.Type] = append(trend[s.Type], s.Key.Duration)
			// all signals here are expected to have the same price
			price = s.Price
			precision += s.Precision
		}
		if len(trend[model.Buy]) > 0 && len(trend[model.Sell]) == 0 {
			signal.Type = model.Buy
		} else if len(trend[model.Sell]) > 0 && len(trend[model.Buy]) == 0 {
			signal.Type = model.Sell
		}
		signal.Key.Coin = trade.Coin
		signal.Price = price
		signal.Precision = precision / float64(len(signals))
		// TODO : get buy or sell from combination of signals
		signal.Key.Duration = 0
		signal.Weight = len(signals)
		k := model.Key{
			Coin:     signal.Key.Coin,
			Duration: signal.Key.Duration,
			Strategy: strategyBuffer.String(),
		}
		signal.Key = k
		// if we get the go-ahead from the strategy act on it
		if act {
			str.signal(k, signal)
		} else {
			log.Info().
				Str("signals", fmt.Sprintf("%+v", signals)).
				Msg("ignoring signals")
		}
	}
	return str.trade(trade)
}

// trade assesses the current trade event and builds up the current state
func (str *strategy) trade(trade *model.Trade) (Signal, model.Key, bool, bool) {
	str.lock.RLock()
	defer str.lock.RUnlock()
	// we want to have buffer time of 4h to evaluate the signal
	for key, s := range str.signals {
		if key.Match(trade.Coin) && str.enabled[key.Coin] {
			str.trades[key] = *trade
			cfg := str._key(key)
			// if we have a signal from the past already ...
			lag := trade.Time.Sub(s.Time).Hours()
			if lag >= cfg.bufferTime {
				// and the signal seems to come true ...
				diff := trade.Price - s.Price
				var act bool
				switch s.Type {
				case model.Buy:
					act = diff >= cfg.priceThreshold
				case model.Sell:
					act = diff <= -1*cfg.priceThreshold
				}
				if act {
					s.Time = trade.Time
					s.Price = trade.Price
					// re-validate the signal
					str.signals[key] = s
					// act on the signal
					return s, key, s.Weight > 0, str.enabled[key.Coin]
				} else {
					log.Debug().
						Float64("lag in hours", lag).
						Float64("diff", diff).
						Str("type", s.Type.String()).
						Msg("ignoring log")
				}
			}
		}
	}
	return Signal{}, model.Key{}, false, false
}

// signal assesses the current signal validity
func (str *strategy) signal(key model.Key, signal Signal) (Signal, bool) {
	replace := true
	if s, ok := str.signals[key]; ok {
		if s.Type != signal.Type {
			replace = true
		} else {
			replace = false
		}
	}
	if replace {
		log.Debug().
			Str("current", fmt.Sprintf("%+v", str.signals[key].ToString())).
			Str("next", fmt.Sprintf("%+v", signal.ToString())).
			Msg("replacing signal")
		str.signals[key] = signal
	} else {
		log.Debug().
			Str("current", fmt.Sprintf("%+v", str.signals[key].ToString())).
			Str("ignore", fmt.Sprintf("%+v", signal.ToString())).
			Msg("keeping signal")
	}
	return signal, signal.Weight > 0
}

// reset assesses the current signal validity
func (str *strategy) reset(key model.Key) bool {
	if _, ok := str.signals[key]; ok {
		delete(str.signals, key)
		return true
	}
	return false
}

func (str strategy) _key(key model.Key) config {
	if cfg, ok := str.config[key]; ok {
		return cfg
	}
	for k, cfg := range str.config {
		if k.Coin == key.Coin {
			return cfg
		}
	}
	return config{}
}
