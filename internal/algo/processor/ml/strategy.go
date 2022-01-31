package ml

import (
	"fmt"
	"sync"

	"github.com/drakos74/free-coin/client"

	"github.com/drakos74/free-coin/internal/time"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/model"
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
	report  map[model.Key]client.Report
	signals map[model.Key]Signal
	trades  map[model.Key]model.Trade
	config  map[model.Key]config
	live    map[model.Coin]bool
}

func newStrategy(segments map[model.Key]Segments) *strategy {
	cfg := make(map[model.Key]config)
	for k, segment := range segments {
		cfg[k] = config{
			bufferTime:     segment.Trader.BufferTime,
			priceThreshold: segment.Trader.PriceThreshold,
			weight:         segment.Trader.Weight,
			enabled:        true,
		}
		log.Info().
			Str("key", fmt.Sprintf("%+v", k)).
			Str("config", fmt.Sprintf("%+v", cfg[k])).
			Msg("init strategy")
	}

	return &strategy{
		lock:    new(sync.RWMutex),
		report:  make(map[model.Key]client.Report),
		signals: make(map[model.Key]Signal),
		trades:  make(map[model.Key]model.Trade),
		config:  cfg,
		live:    make(map[model.Coin]bool),
	}
}

func (str *strategy) enable(c model.Coin, enabled bool) []bool {
	str.lock.Lock()
	defer str.lock.Unlock()
	for k, cfg := range str.config {
		if c == model.AllCoins || k.Match(c) {
			cfg.enabled = enabled
			str.config[k] = cfg
		}
	}
	ee := make([]bool, 0)
	for k, cfg := range str.config {
		if c == model.AllCoins || k.Match(c) {
			ee = append(ee, cfg.enabled)
		}
	}
	return ee
}

func (str *strategy) isLive(trade *model.Trade) bool {
	if str.live[trade.Coin] {
		return true
	}
	if time.IsValidTime(trade.Time) {
		log.Info().Time("stamp", trade.Time).Str("coin", string(trade.Coin)).Msg("strategy going live")
		str.live[trade.Coin] = true
		return true
	}
	return false
}

// trade assesses the current trade event and builds up the current state
func (str *strategy) trade(trade *model.Trade) (Signal, model.Key, bool, bool) {
	str.lock.RLock()
	defer str.lock.RUnlock()
	// we want to have buffer time of 4h to evaluate the signal
	for key, s := range str.signals {
		if key.Match(trade.Coin) && str.config[key].enabled {
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
					act = diff > cfg.priceThreshold
				case model.Sell:
					act = diff < -1*cfg.priceThreshold
				}
				if act {
					s.Time = trade.Time
					s.Price = trade.Price
					// re-validate the signal
					str.signals[key] = s
					// act on the signal
					return s, key, s.Weight > 0, cfg.enabled
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
