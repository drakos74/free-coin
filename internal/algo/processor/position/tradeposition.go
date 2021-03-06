package position

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/emoji"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

var (
	OpenPositionRegistryKey  = fmt.Sprintf("%s-%s", ProcessorName, "open")
	ClosePositionRegistryKey = fmt.Sprintf("%s-%s", ProcessorName, "close")
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
	position model.TrackedPosition
	config   processor.Strategy
}

type tradePositions struct {
	logger         storage.Registry
	pos            map[model.Coin]map[string]*tradePosition
	initialConfigs map[model.Coin]map[time.Duration]processor.Config
	lock           *sync.RWMutex
}

func newPositionTracker(registry storage.Registry, configs map[model.Coin]map[time.Duration]processor.Config) *tradePositions {
	return &tradePositions{
		logger:         registry,
		pos:            make(map[model.Coin]map[string]*tradePosition),
		initialConfigs: configs,
		lock:           new(sync.RWMutex),
	}
}

// TODO : disable user adjustments for now
//func (tp *tradePositions) updateConfig(key tpKey, profit, stopLoss float64) {
//	tp.lock.Lock()
//	defer tp.lock.Unlock()
//	tp.pos[key.coin][key.id].config.Profit.Min = profit
//	tp.pos[key.coin][key.id].config.Loss.Min = stopLoss
//}

type cKey struct {
	cid   string
	coin  model.Coin
	txids []string
}

func (tp *tradePositions) track(client api.Exchange, user api.User, ticker *time.Ticker, quit chan struct{}, block api.Block) {
	// and update the positions at the predefined interval.
	for {
		select {
		case action := <-block.Action: // trigger update on the positions on external events/actions
			// check if we need to act on the action
			// do we have a correlation key ?
			ck := cKey{}
			switch action.Name {
			case model.OrderKey:
				// before we ope lets check what we have ...
				if order, ok := action.Content.(model.Order); ok {

					ck.cid = order.CID
					ck.coin = order.Coin

					correlatedPositions := tp.get(ck)
					// TODO : look closer into the correlated positions ...

					txIDs, err := client.OpenOrder(order)
					if err == nil {
						// Store for correlation and auditing
						trackingOrder := model.TrackingOrder{
							Order: order,
							TxIDs: txIDs,
						}
						err := tp.logger.Put(storage.K{
							Pair:  string(order.Coin),
							Label: OpenPositionRegistryKey,
						}, trackingOrder)
						if err != nil {
							log.Warn().Err(err).
								Str("processor", ProcessorName).
								Str("event", fmt.Sprintf("%+v", trackingOrder)).
								Msg("could not save event to registry")
						}
					}
					// add the txIDs to the key
					ck.txids = txIDs
					log.Info().
						Err(err).
						Str("ID", order.ID).
						Strs("TxIDs", txIDs).
						Int("correlated-positions", len(correlatedPositions)).
						Str("Coin", string(order.Coin)).
						Str("type", order.Type.String()).
						Float64("volume", order.Volume).
						Str("leverage", order.Leverage.String()).
						Msg("submit order")
					api.Reply(api.Private, user, api.
						NewMessage("action").
						AddLine(fmt.Sprintf("open %s %s %.2f",
							order.Type.String(),
							order.Coin,
							order.Volume,
						)).
						AddLine(fmt.Sprintf("%s <-> %v", order.ID, txIDs)), err)
				}
			}
			// update and inject the newly created order
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
			}
			if ck.cid != "" {
				err = tp.inject(ck)
				if err != nil {
					log.Warn().
						Str("cKey", fmt.Sprintf("%+v", ck)).
						Msg("could not correlate new event to any positions")
				}
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

func (tp *tradePositions) inject(ck cKey) error {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	// go over the positions and assign the ck.cid when we have a match for the txids
	if pos, ok := tp.pos[ck.coin]; ok {
		var found bool
		for _, p := range pos {
			// TODO : this is temporary, until we identify what kraken gives us
			// check if its the order id that matched
			for _, txid := range ck.txids {
				if p.position.OrderID == txid || p.position.TxID == txid {
					p.position.CID = ck.cid
				}
			}
		}
		if found {
			return nil
		}
		return fmt.Errorf("no match found for txids: %v", ck.txids)
	} else {
		return fmt.Errorf("no positions found for coin: %s", ck.coin)
	}
}

func (tp *tradePositions) get(ck cKey) []tradePosition {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	positions := make([]tradePosition, 0)
	if pos, ok := tp.pos[ck.coin]; ok {
		var found bool
		for _, p := range pos {
			if p.position.CID == ck.cid {
				positions = append(positions, *p)
			}
		}
		if found {
			return nil
		}
		return positions
	} else {
		return positions
	}
}

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
				position: model.TrackedPosition{
					Open:     oldPosition.position.Open,
					Position: p,
				},
				config: oldPosition.config,
			}
		} else {
			log.Warn().
				Str("pid", p.ID).
				Str("coin", string(p.Coin)).
				Msg("unknown position encountered")
			cfg := tp.getConfiguration(p.Coin)
			tp.pos[p.Coin][p.ID] = &tradePosition{
				position: model.TrackedPosition{
					Open:     p.OpenTime,
					Position: p,
				},
				config: processor.GetAny(cfg),
			}
		}
		posIDs[p.ID] = struct{}{}
	}
	// remove old positions
	toDel := make([]tpKey, 0)
	for c, ps := range tp.pos {
		for id := range ps {
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

func (tp *tradePositions) getConfiguration(coin model.Coin) map[time.Duration]processor.Config {
	if cfg, ok := tp.initialConfigs[coin]; ok {
		return cfg
	}
	return map[time.Duration]processor.Config{}
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
	delete(tp.pos[k.coin], k.id)
	log.Debug().Str("coin", string(k.coin)).Str("id", k.id).Msg("deleted position")
}

func (tp *tradePositions) checkClose(trade *model.Trade) []tradeAction {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	actions := make([]tradeAction, 0)
	if positions, ok := tp.pos[trade.Coin]; ok {
		log.Trace().
			Time("server-time", time.Now()).
			Time("trade-time", trade.Time).
			Int("count", len(positions)).
			Str("coin", string(trade.Coin)).
			Msg("open positions")
		for id, p := range positions {
			p.position.CurrentPrice = trade.Price
			if trade.Live {
				net, profit := p.position.Value()
				log.Debug().
					Str("ID", id).
					Str("coin", string(p.position.Coin)).
					Float64("open", p.position.OpenPrice).
					Float64("current", p.position.CurrentPrice).
					Str("type", p.position.Type.String()).
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

// DoClose checks if the given position should be closed, based on the current configuration.
// TODO : test this logic
func (tp *tradePosition) DoClose() bool {
	net, p := tp.position.Value()
	if net > 0 && p > tp.config.Close.Profit.Min {
		if tp.config.Close.Profit.Trail > 0 {
			// check the previous profit in order to extend profit
			if p > tp.config.Close.Profit.High {
				// if we are making more ... ignore
				tp.config.Close.Profit.High = p
				return false
			}
			diff := tp.config.Close.Profit.High - p
			// TODO :define this in the config as well
			if diff < tp.config.Close.Profit.Trail {
				// leave for now, hoping profit will go up again
				// but dont update our highest value
				return false
			}
		}
		// only close if the market is going down
		return true
	} else if net < 0 && p < -1*tp.config.Close.Loss.Min {
		if p > tp.config.Close.Loss.High {
			tp.config.Close.Loss.High = p
			// we are improving our position ... so give it a bit of time.
			return false
		}
		tp.config.Close.Loss.High = p
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
	err := client.ClosePosition(position.Position)
	position.Close = time
	if err != nil {
		log.Error().
			Err(err).
			Float64("volume", position.Volume).
			Str("id", key.id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("could not close position")
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("could not close %s [%s]: %s",
			key.coin,
			key.id,
			err.Error(),
		)).ReferenceTime(time), nil)
		return false
	} else {
		err := tp.logger.Put(storage.K{
			Pair:  string(position.Coin),
			Label: ClosePositionRegistryKey,
		}, position)
		log.Info().
			Err(err).
			Float64("volume", position.Volume).
			Str("id", key.id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("closed position")
		tp.delete(key)
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("%s closed %s ( %.2f | %.2f%s )",
			emoji.MapToSign(profit),
			key.coin,
			position.Volume,
			profit,
			"%"),
		).ReferenceTime(time), nil)
		return true
	}
}
