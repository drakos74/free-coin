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

func (t tradePosition) updateCID(cid string) {
	pos := t.position
	pos.CID = cid
	t.position = pos
}

type tradePositions struct {
	logger         storage.Registry
	pos            map[model.Coin]map[string]*tradePosition
	txIDs          map[model.Coin]map[string][]string
	initialConfigs map[model.Coin]map[time.Duration]processor.Config
	lock           *sync.RWMutex
}

func newPositionTracker(registry storage.Registry, configs map[model.Coin]map[time.Duration]processor.Config) *tradePositions {
	return &tradePositions{
		logger:         registry,
		pos:            make(map[model.Coin]map[string]*tradePosition),
		txIDs:          make(map[model.Coin]map[string][]string),
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
			// TODO : check on the strategy etc ...
			case model.OrderKey:
				// before we ope lets check what we have ...
				if order, ok := action.Content.(model.Order); ok {

					ck.cid = order.CID
					ck.coin = order.Coin

					// TODO : do this in tp.checkOpen
					correlatedPositions := tp.get(ck)
					log.Info().Int("found", len(correlatedPositions)).Msg("correlated positions")
					if len(correlatedPositions) > 0 {
						// check how many are same direction and how many other direction
						var same int
						var opp int
						for _, cpos := range correlatedPositions {
							if cpos.position.Type == order.Type {
								same++
							} else if cpos.position.Type.Inv() == order.Type {
								opp++
							}
						}
						log.Info().
							Int("same", same).
							Int("opposite", opp).
							Msg("correlation position")
					}

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
						NewMessage(processor.Audit(ProcessorName, order.CID)).
						AddLine(fmt.Sprintf("open %s %s %.3f",
							emoji.MapType(order.Type),
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
					err = tp.deferInject(ck)
					log.Warn().
						Err(err).
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

// get returns the position for the given correlation key
func (tp *tradePositions) get(ck cKey) []tradePosition {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	positions := make([]tradePosition, 0)
	log.Info().Int("size", len(tp.pos[ck.coin])).Msg("checking positions")
	if pos, ok := tp.pos[ck.coin]; ok {
		var found bool
		for _, p := range pos {
			log.Info().
				Str("coin", string(p.position.Coin)).
				Str("pid", p.position.ID).
				Msg("current position")
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

// deferInject defers the injection for later ...
// it needs to be picked up during the update though.
func (tp *tradePositions) deferInject(ck cKey) error {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	if _, ok := tp.txIDs[ck.coin]; ok {
		return fmt.Errorf("current cKey '%+v' is already present '%+v'", ck, tp.txIDs[ck.coin])
	}
	tp.txIDs[ck.coin] = map[string][]string{
		ck.cid: ck.txids,
	}
	return nil
}

// inject injects the given correlation key to the corresponding position.
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
					p.updateCID(ck.cid)
					found = true
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

// update updates the current in-memory positions.
// it needs to check the deferred correlation keys to be injected.
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
		// check the correlation keys
		if txIDs, ok := tp.txIDs[p.Coin]; ok {
			var found bool
			for cid, txID := range txIDs {
				for _, txid := range txID {
					if p.TxID == txid || p.OrderID == txid {
						p.CID = cid
						log.Info().Str("cid", cid).Str("pid", p.ID).Msg("matched cid")
						found = true
					}
				}
			}
			if found {
				tp.txIDs[p.Coin] = make(map[string][]string)
			} else {
				log.Info().
					Str("pid", p.ID).
					Str("txids", fmt.Sprintf("%+v", txIDs)).
					Str("pid", p.ID).
					Msg("could not match")

			}
		}
		// check if position exists
		if oldPosition, ok := tp.pos[p.Coin][p.ID]; ok {
			if p.CID == "" {
				// we did not find anything to inject
				if oldPosition.position.CID != "" {
					// and the old position has a correlation id
					p.CID = oldPosition.position.CID
				}
			}
			tp.pos[p.Coin][p.ID] = &tradePosition{
				position: model.TrackedPosition{
					Open:     oldPosition.position.Open,
					Position: p,
				},
				config: oldPosition.config,
			}
		} else {
			// TODO pick the right strategy as config ... based on the cid
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
				config: processor.GetAny(cfg, p.CID),
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
		user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, "error")).
			AddLine(fmt.Sprintf("could not close %s [%s]: %s",
				key.coin,
				key.id,
				err.Error(),
			)).
			ReferenceTime(time), nil)
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
		user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, "")).
			AddLine(fmt.Sprintf("%s closed %s ( %.3f | %.2f%s )",
				emoji.MapToSign(profit),
				key.coin,
				position.Volume,
				profit,
				"%")).
			ReferenceTime(time), nil)
		return true
	}
}
