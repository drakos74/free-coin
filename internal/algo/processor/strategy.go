package processor

import (
	"fmt"
	"sync"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

// Strategy is responsible for making a call on weather the signal is good enough to trade on.
type Strategy struct {
	lock    *sync.RWMutex
	signals map[model.Key]mlmodel.Signal
	trades  map[model.Key]model.Tick
	config  *mlmodel.Config
	live    map[model.Coin]bool
	enabled map[model.Coin]bool
}

func NewStrategy(segments *mlmodel.Config) *Strategy {
	return &Strategy{
		lock:    new(sync.RWMutex),
		signals: make(map[model.Key]mlmodel.Signal),
		trades:  make(map[model.Key]model.Tick),
		config:  segments,
		live:    make(map[model.Coin]bool),
	}
}

func (s *Strategy) Config() mlmodel.Config {
	return *s.config
}

func (s *Strategy) SetGap(coin model.Coin, gap float64) mlmodel.Config {
	s.config.SetGap(coin, gap)
	return s.Config()
}

func (s *Strategy) SetPrecisionThreshold(coin model.Coin, network string, precision float64) mlmodel.Config {
	s.config.SetPrecisionThreshold(coin, network, precision)
	return s.Config()
}

func (s *Strategy) EnableML(c model.Coin, enabled bool) map[string]bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	newConfig := &mlmodel.Config{
		Segments: s.config.Segments,
		Position: s.config.Position,
		Option:   s.config.Option,
		Buffer:   s.config.Buffer,
	}
	ee := make(map[string]bool, 0)
	for k, cfg := range s.config.Segments {
		if c == model.AllCoins || k.Match(c) {
			cfg.Stats.Live = enabled
			newConfig.Segments[k] = cfg
			ee[k.ToString()] = newConfig.Segments[k].Stats.Live
		}
	}
	s.config = newConfig
	return ee
}

func (s *Strategy) EnableTrader(c model.Coin, enabled bool) map[string]bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	newConfig := &mlmodel.Config{
		Segments: s.config.Segments,
		Position: s.config.Position,
		Option:   s.config.Option,
		Buffer:   s.config.Buffer,
	}
	ee := make(map[string]bool, 0)
	for k, cfg := range s.config.Segments {
		if c == model.AllCoins || k.Match(c) {
			cfg.Trader.Live = enabled
			newConfig.Segments[k] = cfg
			ee[k.ToString()] = newConfig.Segments[k].Trader.Live
		}
	}
	s.config = newConfig
	return ee
}

func (s *Strategy) IsLive(coin model.Coin, trade model.Tick) (bool, bool) {
	if s.live[coin] {
		return true, false
	}
	if cointime.IsValidTime(trade.Time) {
		log.Info().Time("stamp", trade.Time).Str("coin", string(coin)).Msg("Strategy going live")
		s.live[coin] = true
		return true, true
	}
	return false, false
}

func (s *Strategy) Eval(trade model.Tick, signal mlmodel.Signal, config *mlmodel.Config) (mlmodel.Signal, model.Key, bool, bool) {
	if signal.Type == model.NoType {
		log.Error().Str("signal", fmt.Sprintf("%+v", signal)).Msg("wrong signal")
		return mlmodel.Signal{}, model.Key{}, false, false
	}
	// Strategy is not live yet ... trade is past one
	if live, _ := s.IsLive(signal.Key.Coin, trade); !live && !config.Option.Debug {
		return mlmodel.Signal{}, model.Key{}, false, false
	}
	k := model.Key{
		Coin: signal.Key.Coin,
	}
	signal.Key = k
	// if we get the go-ahead from the Strategy act on it
	s.signal(k, signal)
	return s.trade(k, trade)
}

// trade assesses the current trade event and builds up the current state
func (s *Strategy) trade(k model.Key, trade model.Tick) (mlmodel.Signal, model.Key, bool, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	// we want to have buffer time of 4h to evaluate the signal
	for key, signal := range s.signals {
		if key.Match(k.Coin) && (s.config.Segments[key].Trader.Live || s.config.Option.Debug) {
			s.trades[key] = trade
			cfg := s._key(key)
			// if we have a signal from the past already ...
			lag := trade.Time.Sub(signal.Time).Hours()
			if lag >= cfg.BufferTime {
				// and the signal seems to come true ...
				diff := trade.Price - signal.Price
				var act bool = true
				//switch s.Detail {
				//case model.Buy:
				//	act = diff >= cfg.priceThreshold
				//case model.Sell:
				//	act = diff <= -1*cfg.priceThreshold
				//}
				if act {
					signal.Time = trade.Time
					signal.Price = trade.Price
					// re-validate the signal
					s.signals[key] = signal
					// act on the signal
					return signal, key, signal.Weight > 0, s.config.Segments[key].Trader.Live || s.config.Option.Debug
				} else {
					log.Debug().
						Float64("lag in hours", lag).
						Float64("diff", diff).
						Str("type", signal.Type.String()).
						Msg("ignoring Tick")
				}
			}
		}
	}
	return mlmodel.Signal{}, model.Key{}, false, false
}

// signal assesses the current signal validity
func (s *Strategy) signal(key model.Key, signal mlmodel.Signal) (mlmodel.Signal, bool) {
	replace := true
	if this_signal, ok := s.signals[key]; ok {
		if this_signal.Type != signal.Type {
			replace = true
		} else {
			replace = false
		}
	}
	if replace {
		log.Debug().
			Str("current", fmt.Sprintf("%+v", s.signals[key].ToString())).
			Str("next", fmt.Sprintf("%+v", signal.ToString())).
			Msg("replacing signal")
		s.signals[key] = signal
	} else {
		log.Debug().
			Str("current", fmt.Sprintf("%+v", s.signals[key].ToString())).
			Str("ignore", fmt.Sprintf("%+v", signal.ToString())).
			Msg("keeping signal")
	}
	return signal, signal.Weight > 0
}

// Reset assesses the current signal validity
func (s *Strategy) Reset(key model.Key) bool {
	if _, ok := s.signals[key]; ok {
		delete(s.signals, key)
		return true
	}
	return false
}

func (s Strategy) _key(key model.Key) mlmodel.Trader {
	if cfg, ok := s.config.Segments[key]; ok {
		return cfg.Trader
	}
	for k, cfg := range s.config.Segments {
		if k.Coin == key.Coin {
			return cfg.Trader
		}
	}
	return mlmodel.Trader{}
}
