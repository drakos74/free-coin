package ml

import (
	"fmt"
	"sync"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/algo/processor/ml/net"
	"github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

// strategy is responsible for making a call on weather the signal is good enough to trade on.
type strategy struct {
	lock     *sync.RWMutex
	signals  map[model.Key]mlmodel.Signal
	trades   map[model.Key]model.Tick
	config   *mlmodel.Config
	datasets *net.Datasets
	live     map[model.Coin]bool
}

func newStrategy(segments *mlmodel.Config, datasets *net.Datasets) *strategy {
	return &strategy{
		lock:     new(sync.RWMutex),
		signals:  make(map[model.Key]mlmodel.Signal),
		trades:   make(map[model.Key]model.Tick),
		config:   segments,
		datasets: datasets,
		live:     make(map[model.Coin]bool),
	}
}

func (str *strategy) enable(c model.Coin, enabled bool) map[string]bool {
	str.lock.Lock()
	defer str.lock.Unlock()
	newConfig := &mlmodel.Config{
		Segments: str.config.Segments,
		Position: str.config.Position,
		Option:   str.config.Option,
		Buffer:   str.config.Buffer,
	}
	ee := make(map[string]bool, 0)
	for k, cfg := range str.config.Segments {
		if c == model.AllCoins || k.Match(c) {
			cfg.Trader.Live = enabled
			newConfig.Segments[k] = cfg
			ee[k.ToString()] = newConfig.Segments[k].Trader.Live
		}
	}
	str.config = newConfig
	return ee
}

func (str *strategy) isLive(coin model.Coin, trade model.Tick) (bool, bool) {
	if str.live[coin] {
		return true, false
	}
	if cointime.IsValidTime(trade.Time) {
		log.Info().Time("stamp", trade.Time).Str("coin", string(coin)).Msg("strategy going live")
		str.live[coin] = true
		return true, true
	}
	return false, false
}

func (str *strategy) eval(trade model.Tick, signal mlmodel.Signal, config *mlmodel.Config) (mlmodel.Signal, model.Key, bool, bool) {
	if signal.Type == model.NoType {
		log.Error().Str("signal", fmt.Sprintf("%+v", signal)).Msg("wrong signal")
		return mlmodel.Signal{}, model.Key{}, false, false
	}
	// strategy is not live yet ... trade is past one
	if live, _ := str.isLive(signal.Key.Coin, trade); !live && !config.Option.Debug {
		return mlmodel.Signal{}, model.Key{}, false, false
	}
	k := model.Key{
		Coin: signal.Key.Coin,
	}
	signal.Key = k
	// if we get the go-ahead from the strategy act on it
	str.signal(k, signal)
	return str.trade(k, trade)
}

// trade assesses the current trade event and builds up the current state
func (str *strategy) trade(k model.Key, trade model.Tick) (mlmodel.Signal, model.Key, bool, bool) {
	str.lock.RLock()
	defer str.lock.RUnlock()
	// we want to have buffer time of 4h to evaluate the signal
	for key, s := range str.signals {
		if key.Match(k.Coin) && (str.config.Segments[key].Trader.Live || str.config.Option.Debug) {
			str.trades[key] = trade
			cfg := str._key(key)
			// if we have a signal from the past already ...
			lag := trade.Time.Sub(s.Time).Hours()
			if lag >= cfg.BufferTime {
				// and the signal seems to come true ...
				diff := trade.Price - s.Price
				var act bool = true
				//switch s.Type {
				//case model.Buy:
				//	act = diff >= cfg.priceThreshold
				//case model.Sell:
				//	act = diff <= -1*cfg.priceThreshold
				//}
				if act {
					s.Time = trade.Time
					s.Price = trade.Price
					// re-validate the signal
					str.signals[key] = s
					// act on the signal
					return s, key, s.Weight > 0, str.config.Segments[key].Trader.Live || str.config.Option.Debug
				} else {
					log.Debug().
						Float64("lag in hours", lag).
						Float64("diff", diff).
						Str("type", s.Type.String()).
						Msg("ignoring Tick")
				}
			}
		}
	}
	return mlmodel.Signal{}, model.Key{}, false, false
}

// signal assesses the current signal validity
func (str *strategy) signal(key model.Key, signal mlmodel.Signal) (mlmodel.Signal, bool) {
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

func (str strategy) _key(key model.Key) mlmodel.Trader {
	if cfg, ok := str.config.Segments[key]; ok {
		return cfg.Trader
	}
	for k, cfg := range str.config.Segments {
		if k.Coin == key.Coin {
			return cfg.Trader
		}
	}
	return mlmodel.Trader{}
}
