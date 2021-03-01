package position

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type tpKey struct {
	coin model.Coin
	id   string
}

func key(c model.Coin, id string) tpKey {
	return tpKey{
		coin: c,
		id:   id,
	}
}

type tradePosition struct {
	position model.Position
	config   Config
}

type tradePositions struct {
	pos            map[model.Coin]map[string]*tradePosition
	initialConfigs []Config
	lock           *sync.RWMutex
}

func (tp *tradePositions) checkClose(trade *model.Trade) []tradeAction {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	actions := make([]tradeAction, 0)
	if positions, ok := tp.pos[trade.Coin]; ok {
		log.Debug().
			Time("server-time", time.Now()).
			Time("trade-time", trade.Time).
			Int("count", len(positions)).
			Str("coin", string(trade.Coin)).
			Msg("open positions")
		for id, p := range positions {
			p.position.CurrentPrice = trade.Price
			if trade.Live {
				net, profit := p.position.Value()
				log.Trace().
					Str("ID", id).
					Str("coin", string(p.position.Coin)).
					Float64("net", net).
					Float64("profit", profit).
					Msg("check position")
				actions = append(actions, tradeAction{
					key:      key(trade.Coin, id),
					position: p,
					doClose:  p.DoClose(),
				})
			}
		}
	}
	for _, action := range actions {
		tp.pos[action.key.coin][action.key.id] = action.position
	}
	return actions
}

// TODO : disable user adjustments for now
//func (tp *tradePositions) updateConfig(key tpKey, profit, stopLoss float64) {
//	tp.lock.Lock()
//	defer tp.lock.Unlock()
//	tp.pos[key.coin][key.id].config.Profit.Min = profit
//	tp.pos[key.coin][key.id].config.Loss.Min = stopLoss
//}

func (tp *tradePositions) update(client api.Exchange) error {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	pp, err := client.OpenPositions(context.Background())
	if err != nil {
		return fmt.Errorf("could not get positions: %w", err)
	}
	posIDs := make(map[string]struct{})
	for _, p := range pp.Positions {
		if _, ok := tp.pos[p.Coin]; !ok {
			tp.pos[p.Coin] = make(map[string]*tradePosition)
		}
		// check if position exists
		if oldPosition, ok := tp.pos[p.Coin][p.ID]; ok {
			tp.pos[p.Coin][p.ID] = &tradePosition{
				position: p,
				config:   oldPosition.config,
			}
		} else {
			cfg := tp.getConfiguration(p.Coin)
			tp.pos[p.Coin][p.ID] = &tradePosition{
				position: p,
				config:   cfg,
			}
		}
		posIDs[p.ID] = struct{}{}
	}
	// remove old positions
	toDel := make([]tpKey, 0)
	for c, ps := range tp.pos {
		for id, _ := range ps {
			if _, ok := posIDs[id]; !ok {
				toDel = append(toDel, tpKey{
					coin: c,
					id:   id,
				})
			}
		}
	}
	for _, del := range toDel {
		delete(tp.pos[del.coin], del.id)
	}
	return nil
}

func (tp *tradePositions) getConfiguration(coin model.Coin) Config {
	var defaultConfig Config
	for _, cfg := range tp.initialConfigs {
		if cfg.Coin == "" {
			defaultConfig = cfg
		}
		if coin == model.Coin(cfg.Coin) {
			return cfg
		}
	}
	return defaultConfig
}

func (tp *tradePositions) track(client api.Exchange, ticker *time.Ticker, quit chan struct{}, block api.Block) {
	// and update the positions at the predefined interval.
	for {
		select {
		case <-block.Action: // trigger update on the positions on external events/actions
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
			}
			block.ReAction <- api.Action{}
		case <-ticker.C: // trigger an update on the positions at regular intervals
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
			}
		case <-quit: // stop the tracking of positions
			ticker.Stop()
			return
		}
	}
}

func (tp *tradePositions) getAll() map[model.Coin]map[string]tradePosition {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	positions := make(map[model.Coin]map[string]tradePosition)
	for c, pp := range tp.pos {
		positions[c] = make(map[string]tradePosition)
		for id, p := range pp {
			positions[c][id] = *p
		}
	}
	return positions
}

func (tp *tradePositions) delete(k tpKey) {
	//ttp := make(map[model.Coin]map[string]*tradePosition)
	//for c, positions := range tp.pos {
	//	if _, ok := ttp[c]; !ok {
	//		ttp[c] = make(map[string]*tradePosition)
	//	}
	//	for id, pos := range positions {
	//		key := key(c, id)
	//		if key != k {
	//			ttp[c][id] = pos
	//		}
	//	}
	//}
	//tp.pos = ttp
	delete(tp.pos[k.coin], k.id)
	log.Debug().Str("coin", string(k.coin)).Str("id", k.id).Msg("deleted position")
}

func (tp *tradePositions) exists(key tpKey) bool {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	// ok, here we might encounter the case that we closed the position from another trigger
	if _, ok := tp.pos[key.coin]; !ok {
		return false
	}
	if _, ok := tp.pos[key.coin][key.id]; !ok {
		return false
	}
	return true
}

// DoClose checks if the given position should be closed, based on the current configuration.
// TODO : test this logic
func (tp *tradePosition) DoClose() bool {
	net, p := tp.position.Value()
	if net > 0 && p > tp.config.Profit.Min {
		// check the previous profit in order to extend profit
		if p > tp.config.Profit.High {
			// if we are making more ... ignore
			tp.config.Profit.High = p
			return false
		}
		diff := tp.config.Profit.High - p
		// TODO :define this in the config as well
		if diff < 0.15 {
			// leave for now, hoping profit will go up again
			// but dont update our highest value
			return false
		}
		// only close if the market is going down
		return true
	}
	if net < 0 && p < -1*tp.config.Loss.Min {
		if p > tp.config.Loss.High {
			tp.config.Loss.High = p
			// we are improving our position ... so give it a bit of time.
			return false
		}
		tp.config.Loss.High = p
		// only close if the market is going up
		return true
	}
	// check if we missed a profit opportunity here
	return false
}

func (tp *tradePositions) close(client api.Exchange, user api.User, key tpKey, time time.Time) bool {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	// ok, here we might encounter the case that we closed the position from another trigger
	if _, ok := tp.pos[key.coin]; !ok {
		return false
	}
	if _, ok := tp.pos[key.coin][key.id]; !ok {
		return false
	}
	position := tp.pos[key.coin][key.id].position
	net, profit := position.Value()
	err := client.ClosePosition(position)
	if err != nil {
		log.Error().
			Float64("volume", position.Volume).
			Str("id", key.id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("could not close position")
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("could not close %s [%s]: %s", key.coin, key.id, err.Error())).ReferenceTime(time), nil)
		return false
	} else {
		log.Info().
			Float64("volume", position.Volume).
			Str("id", key.id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("closed position")
		tp.delete(key)
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("closed %s at %.2f [%s]", key.coin, profit, key.id)).ReferenceTime(time), nil)
		return true
	}
}
