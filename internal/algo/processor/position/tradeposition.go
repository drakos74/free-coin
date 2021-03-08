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

func newPositionTracker(shard storage.Shard, registry storage.Registry, configs map[model.Coin]map[time.Duration]processor.Config) *tradePositions {

	// load the book right at start-up
	state, err := shard(ProcessorName)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		state = storage.NewVoidStorage()
	}
	positions := make(map[model.Coin]Portfolio)
	err = state.Load(stateKey, positions)
	log.Info().Err(err).Int("book", len(positions)).Msg("loaded book")

	return &tradePositions{
		registry:       registry,
		state:          state,
		book:           positions,
		txIDs:          make(map[model.Coin]map[string]map[string]struct{}),
		initialConfigs: configs,
		lock:           new(sync.RWMutex),
	}
}

// TODO : disable user adjustments for now
//func (tp *tradePositions) updateConfig(key tpKey, profit, stopLoss float64) {
//	tp.lock.Lock()
//	defer tp.lock.Unlock()
//	tp.book[key.coin][key.id].Config.Profit.Min = profit
//	tp.book[key.coin][key.id].Config.Loss.Min = stopLoss
//}

type cKey struct {
	cid   string
	coin  model.Coin
	txids []string
}

func (tp *tradePositions) track(client api.Exchange, user api.User, ticker *time.Ticker, quit chan struct{}, block api.Block) {
	// and update the book at the predefined interval.
	for {
		select {
		case action := <-block.Action: // trigger update on the book on external events/actions
			// check if we need to act on the action
			// do we have a correlation key ?
			switch action.Name {
			// TODO : check on the strategy etc ...
			case model.OrderKey:
				// before we ope lets check what we have ...
				if order, ok := action.Content.(model.Order); ok {
					ck := cKey{
						cid:  order.CID,
						coin: order.Coin,
					}

					// TODO : do this in tp.checkOpen
					correlatedPositions := tp.get(ck)
					log.Info().Int("found", len(correlatedPositions)).Msg("correlated book")
					if len(correlatedPositions) > 0 {
						// check how many are same direction and how many other direction
						var same int
						var opp int
						for _, cpos := range correlatedPositions {
							if cpos.Position.Type == order.Type {
								same++
							} else if cpos.Position.Type.Inv() == order.Type {
								opp++
							}
						}
						log.Info().
							Int("same", same).
							Int("opposite", opp).
							Msg("correlation Position")
					}

					txIDs, err := client.OpenOrder(order)
					if err == nil {
						// Store for correlation and auditing
						trackingOrder := model.TrackingOrder{
							Order: order,
							TxIDs: txIDs,
						}
						storage.Add(tp.registry, openKey(order.Coin), trackingOrder)
					}
					// add the txIDs to the key
					ck.txids = txIDs
					log.Info().
						Err(err).
						Str("ID", order.ID).
						Strs("TxIDs", txIDs).
						Int("correlated-book", len(correlatedPositions)).
						Str("Coin", string(order.Coin)).
						Str("type", order.Type.String()).
						Float64("volume", order.Volume).
						Str("leverage", order.Leverage.String()).
						Msg("submitted order")
					api.Reply(api.Private, user, api.
						NewMessage(processor.Audit(ProcessorName, order.CID)).
						AddLine(fmt.Sprintf("open %s %s %.3f",
							emoji.MapType(order.Type),
							order.Coin,
							order.Volume,
						)), err)
					added, err := tp.deferInject(ck)
					log.Info().
						Err(err).
						Int("added", added).
						Msg("defer injected cid")
				}
			}
			// update and inject the newly created order
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get book")
			}
			block.ReAction <- api.Action{}
		case <-ticker.C: // trigger an update on the book at regular intervals
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get book")
			}
		case <-quit: // stop the tracking of book
			ticker.Stop()
			return
		}
	}
}

// get returns the Position for the given correlation key
func (tp *tradePositions) get(ck cKey) []TradePosition {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	positions := make([]TradePosition, 0)
	log.Info().Int("size", len(tp.book[ck.coin].Positions)).Msg("checking book")
	if pos, ok := tp.book[ck.coin]; ok {
		var found bool
		for _, p := range pos.Positions {
			log.Info().
				Str("coin", string(p.Position.Coin)).
				Str("pid", p.Position.ID).
				Str("txid", p.Position.TxID).
				Str("order-id", p.Position.OrderID).
				Str("cid", p.Position.CID).
				Msg("current Position")
			if p.Position.CID == ck.cid {
				positions = append(positions, p)
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
func (tp *tradePositions) deferInject(ck cKey) (int, error) {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	if len(ck.txids) == 0 {
		return 0, fmt.Errorf("no txids to inject %+v", ck)
	}
	if _, ok := tp.txIDs[ck.coin]; !ok {
		tp.txIDs[ck.coin] = make(map[string]map[string]struct{})
	}
	if _, ok := tp.txIDs[ck.coin][ck.cid]; !ok {
		tp.txIDs[ck.coin][ck.cid] = make(map[string]struct{})
	}
	var added int
	for _, txid := range ck.txids {
		if _, ok := tp.txIDs[ck.coin][ck.cid][txid]; !ok {
			tp.txIDs[ck.coin][ck.cid][txid] = struct{}{}
			added++
		}
	}
	return added, nil
}

// update updates the current in-memory book.
// it needs to check the deferred correlation keys to be injected.
func (tp *tradePositions) update(client api.Exchange) error {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	pp, err := client.OpenPositions(context.Background())
	if err != nil {
		return fmt.Errorf("could not get book: %w", err)
	}
	posIDs := make(map[string]struct{})
	for _, p := range pp.Positions {
		if _, ok := tp.book[p.Coin]; !ok {
			tp.book[p.Coin] = Portfolio{
				Positions: make(map[string]TradePosition),
			}
		}
		// check the correlation keys
		if txIDs, ok := tp.txIDs[p.Coin]; ok && len(txIDs) > 0 {
			var matched int
			var of int
			for cid, txID := range txIDs {
				for txid := range txID {
					of++
					if p.TxID == txid || p.OrderID == txid {
						p.CID = cid
						log.Info().Str("cid", cid).Str("pid", p.ID).Msg("matched cid")
						matched++
						delete(tp.txIDs[p.Coin][cid], txid)
					}
				}
			}
			log.Info().
				Int("matched", matched).
				Int("of", of).
				Str("txids", fmt.Sprintf("%+v", txIDs)).
				Str("pid", p.ID).
				Msg("correlate Position")
		}
		// check if Position exists
		if oldPosition, ok := tp.book[p.Coin].Positions[p.ID]; ok {
			if p.CID == "" {
				// we did not find anything to inject
				if oldPosition.Position.CID != "" {
					// and the old Position has a correlation id
					p.CID = oldPosition.Position.CID
				}
			}
			tp.book[p.Coin].Positions[p.ID] = TradePosition{
				Position: model.TrackedPosition{
					Open:     oldPosition.Position.Open,
					Position: p,
				},
				Config: oldPosition.Config,
			}
		} else {
			// if p.CID == "" should have happened already in the previous step
			cfg := tp.getConfiguration(p.Coin)
			tp.book[p.Coin].Positions[p.ID] = TradePosition{
				Position: model.TrackedPosition{
					Open:     p.OpenTime,
					Position: p,
				},
				Config: processor.GetAny(cfg, p.CID),
			}
		}
		posIDs[p.ID] = struct{}{}
	}
	// remove old book
	toDel := make([]tpKey, 0)
	for c, portfolio := range tp.book {
		for id := range portfolio.Positions {
			if _, ok := posIDs[id]; !ok {
				toDel = append(toDel, tpKey{
					coin: c,
					id:   id,
				})
			}
		}
	}
	for _, del := range toDel {
		delete(tp.book[del.coin].Positions, del.id)
	}
	// save the state

	storage.Store(tp.state, stateKey, tp.book)
	return nil
}

func (tp *tradePositions) getConfiguration(coin model.Coin) map[time.Duration]processor.Config {
	if cfg, ok := tp.initialConfigs[coin]; ok {
		return cfg
	}
	return map[time.Duration]processor.Config{}
}

func (tp *tradePositions) getAll() map[model.Coin]map[string]TradePosition {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	positions := make(map[model.Coin]map[string]TradePosition)
	for c, portfolio := range tp.book {
		positions[c] = make(map[string]TradePosition)
		for id, p := range portfolio.Positions {
			positions[c][id] = p
		}
	}
	return positions
}

func (tp *tradePositions) delete(k tpKey) {
	delete(tp.book[k.coin].Positions, k.id)
	log.Debug().Str("coin", string(k.coin)).Str("id", k.id).Msg("deleted Position")
}

func (tp *tradePositions) budget(coin model.Coin, net float64) {
	if portfolio, ok := tp.book[coin]; ok {
		portfolio.Budget += net
		tp.book[coin] = portfolio
	} else {
		log.Error().Str("coin", string(coin)).Msg("portfolio not found")
	}
}

func (tp *tradePositions) checkClose(trade *model.Trade) []tradeAction {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	actions := make([]tradeAction, 0)
	if portfolio, ok := tp.book[trade.Coin]; ok {
		log.Trace().
			Time("server-time", time.Now()).
			Time("trade-time", trade.Time).
			Int("count", len(portfolio.Positions)).
			Str("coin", string(trade.Coin)).
			Msg("open portfolio")
		for id, p := range portfolio.Positions {
			p.Position.CurrentPrice = trade.Price
			if trade.Live {
				net, profit := p.Position.Value()
				log.Debug().
					Str("ID", id).
					Str("coin", string(p.Position.Coin)).
					Float64("open", p.Position.OpenPrice).
					Float64("current", p.Position.CurrentPrice).
					Str("type", p.Position.Type.String()).
					Float64("net", net).
					Float64("profit", profit).
					Msg("check Position")
				actions = append(actions, tradeAction{
					key:      key(trade.Coin, id),
					position: p,
					doClose:  p.DoClose(),
				})
			}
		}
	}
	for _, action := range actions {
		tp.book[action.key.coin].Positions[action.key.id] = action.position
	}
	storage.Store(tp.state, stateKey, tp.book)
	return actions
}

// DoClose checks if the given Position should be closed, based on the current configuration.
// TODO : test this logic
func (tp *TradePosition) DoClose() bool {
	net, p := tp.Position.Value()
	if net > 0 && p > tp.Config.Close.Profit.Min {
		if tp.Config.Close.Profit.Trail > 0 {
			// check the previous profit in order to extend profit
			if p > tp.Config.Close.Profit.High {
				// if we are making more ... ignore
				tp.Config.Close.Profit.High = p
				return false
			}
			diff := tp.Config.Close.Profit.High - p
			// TODO :define this in the Config as well
			if diff < tp.Config.Close.Profit.Trail {
				// leave for now, hoping profit will go up again
				// but dont update our highest value
				return false
			}
		}
		// only close if the market is going down
		return true
	} else if net < 0 && p < -1*tp.Config.Close.Loss.Min {
		if p > tp.Config.Close.Loss.High {
			tp.Config.Close.Loss.High = p
			// we are improving our Position ... so give it a bit of time.
			return false
		}
		tp.Config.Close.Loss.High = p
		// only close if the market is going up
		return true
	}
	// check if we missed a profit opportunity here
	return false
}

func (tp *tradePositions) close(client api.Exchange, user api.User, key tpKey, time time.Time) bool {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	// ok, here we might encounter the case that we closed the Position from another trigger
	if _, ok := tp.book[key.coin]; !ok {
		return false
	}
	if _, ok := tp.book[key.coin].Positions[key.id]; !ok {
		return false
	}
	position := tp.book[key.coin].Positions[key.id].Position
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
			Msg("could not close Position")
		user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, "error")).
			AddLine(fmt.Sprintf("could not close %s [%s]: %s",
				key.coin,
				key.id,
				err.Error(),
			)).
			ReferenceTime(time), nil)
		return false
	} else {
		storage.Add(tp.registry, closeKey(position.Coin), position)
		log.Info().
			Float64("volume", position.Volume).
			Str("type", position.Type.String()).
			Str("id", key.id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("closed Position")
		// TODO : would be good if we do all together atomically
		tp.delete(key)
		tp.budget(position.Coin, net)
		storage.Store(tp.state, stateKey, tp.book)
		user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, "")).
			AddLine(fmt.Sprintf("%s closed %s %s ( %.3f | %.2f%s )",
				emoji.MapToSign(profit),
				key.coin,
				emoji.MapType(position.Type),
				position.Volume,
				profit,
				"%")).
			ReferenceTime(time), nil)
		return true
	}
}
