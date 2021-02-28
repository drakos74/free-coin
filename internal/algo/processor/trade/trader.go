package trade

import (
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type trader struct {
	// TODO : improve the concurrency factor. this is temporary though inefficient locking
	lock           sync.RWMutex
	initialConfigs []Config
	configs        map[model.Coin]map[time.Duration]OpenConfig
	logs           map[string]struct{}
}

func newTrader(configs ...Config) *trader {
	return &trader{
		lock:           sync.RWMutex{},
		initialConfigs: configs,
		configs:        make(map[model.Coin]map[time.Duration]OpenConfig),
		logs:           make(map[string]struct{}),
	}
}

func (tr *trader) init(k processor.Key) {
	tr.lock.Lock()
	defer tr.lock.Unlock()
	if _, ok := tr.configs[k.Coin]; !ok {
		tr.configs[k.Coin] = make(map[time.Duration]OpenConfig)
	}
	// this is a trading strategy and we dont want to do too many things by default if not defined.
	if _, ok := tr.configs[k.Coin][k.Duration]; !ok {
		// try to find a config in the initial ones
		var myCfg OpenConfig
		var found bool
		var exists bool
		for _, cfg := range tr.initialConfigs {
			for _, strategy := range cfg.Strategies {
				if cfg.Coin == "" {
					exists = true
					myCfg = OpenConfig{
						MinSample:      strategy.Sample,
						MinProbability: strategy.Probability,
						Value:          cfg.Open.Value,
						Strategy:       getStrategy(strategy.Name, strategy.Threshold),
					}
				}
				if model.Coin(cfg.Coin) == k.Coin && strategy.Target == int(k.Duration.Minutes()) {
					found = true
					exists = true
					config := OpenConfig{
						MinSample:      strategy.Sample,
						MinProbability: strategy.Probability,
						Value:          cfg.Open.Value,
						Strategy:       getStrategy(strategy.Name, strategy.Threshold),
					}
					tr.configs[k.Coin][k.Duration] = config
					log.Info().
						//Str("strategy", fmt.Sprintf("%+v", config)).
						Str("key", fmt.Sprintf("%s", k.String())).
						Msg("init coin strategy")
				}
			}
		}
		if exists {
			if !found {
				tr.configs[k.Coin][k.Duration] = myCfg
			}
			log.Info().
				Bool("exists", exists).
				Bool("default", !found).
				//Str("strategy", fmt.Sprintf("%+v", myCfg)).
				Str("key", fmt.Sprintf("%s", k.String())).
				Msg("init default strategy")
		} else {
			tr.logOnce(fmt.Sprintf("no trade strategy defined for %v", k))
		}
	}
}

func (tr *trader) logOnce(msg string) {
	if _, ok := tr.logs[msg]; !ok {
		log.Error().Msg(msg)
		tr.logs[msg] = struct{}{}
	}
}

func (tr *trader) get(k processor.Key) (OpenConfig, bool) {
	tr.lock.RLock()
	defer tr.lock.RUnlock()
	cfg, ok := tr.configs[k.Coin][k.Duration]
	return cfg, ok
}

func (tr *trader) getAll(c model.Coin) map[time.Duration]OpenConfig {
	tr.lock.RLock()
	defer tr.lock.RUnlock()
	configs := make(map[time.Duration]OpenConfig)
	for d, cfg := range tr.configs[c] {
		configs[d] = cfg
	}
	return configs
}

func (tr *trader) set(k processor.Key, probability float64, sample int) (time.Duration, OpenConfig) {
	tr.init(k)
	tr.lock.Lock()
	defer tr.lock.Unlock()
	cfg := tr.configs[k.Coin][k.Duration]
	cfg.MinProbability = probability
	cfg.MinSample = sample
	tr.configs[k.Coin][k.Duration] = cfg
	return k.Duration, tr.configs[k.Coin][k.Duration]
}
