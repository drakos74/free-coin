package position

import (
	"context"
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

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
				if trackedOrder, ok := action.Content.(model.TrackedOrder); ok {
					// check how many are same direction and how many other direction
					order, ok := tp.checkOpen(trackedOrder.Key, trackedOrder)

					if !ok {
						// dont proceed ...
						break
					}

					txIDs, err := client.OpenOrder(order)
					if err == nil {
						// Store for correlation and auditing
						trackedOrder.TxIDs = txIDs
						storage.Add(tp.registry, openKey(trackedOrder.Coin), trackedOrder)
					}
					// add the txIDs to the key
					log.Info().
						Err(err).
						Str("ID", trackedOrder.ID).
						Strs("TxIDs", txIDs).
						Str("Coin", string(trackedOrder.Coin)).
						Str("type", trackedOrder.Type.String()).
						Float64("volume", trackedOrder.Volume).
						Str("leverage", trackedOrder.Leverage.String()).
						Msg("submitted trackedOrder")
					api.Reply(api.Private, user, api.
						NewMessage(processor.Audit(ProcessorName, trackedOrder.Key.ToString())).
						AddLine(fmt.Sprintf("open %s %s %.3f",
							emoji.MapType(trackedOrder.Type),
							trackedOrder.Coin,
							trackedOrder.Volume,
						)), err)
					added, err := tp.deferInject(trackedOrder.Key, trackedOrder.TxIDs)
					log.Info().
						Err(err).
						Int("added", added).
						Msg("defer injected cid")
				}
			}
			// update and inject the newly created trackedOrder
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
func (tp *tradePositions) get(k model.Key) []TradePosition {
	positions := make([]TradePosition, 0)
	log.Info().Int("size", len(tp.book[k].Positions)).Msg("checking book")
	if pos, ok := tp.book[k]; ok {
		var found bool
		for _, p := range pos.Positions {
			log.Info().
				Str("coin", string(p.Position.Coin)).
				Str("pid", p.Position.ID).
				Str("order-id", p.Position.OrderID).
				Str("cid", p.Position.CID).
				Msg("current Position")
			if p.Position.CID == k.ToString() {
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
func (tp *tradePositions) deferInject(k model.Key, txIDs []string) (int, error) {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	if len(txIDs) == 0 {
		return 0, fmt.Errorf("no txids to inject for %+v", k)
	}
	if _, ok := tp.txIDs[k]; !ok {
		tp.txIDs[k] = make(map[string]struct{})
	}
	var added int
	for _, txid := range txIDs {
		tp.txIDs[k][txid] = struct{}{}
		added++
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
		// check the correlation keys
		var matched int
		var of int
		for k, txIDs := range tp.txIDs {
			for txid := range txIDs {
				of++
				if p.TxID == txid || p.OrderID == txid {
					p.CID = k.ToString()
					log.Info().Str("cid", p.CID).Str("pid", p.ID).Msg("matched cid")
					matched++
					delete(tp.txIDs[k], txid)
				}
			}
		}
		// check if Position exists
		var exists bool
		for k, book := range tp.book {
			if oldPosition, ok := book.Positions[p.ID]; ok {
				if p.CID == "" {
					// we did not find anything to inject
					if oldPosition.Position.CID != "" {
						// and the old Position has a correlation id
						p.CID = oldPosition.Position.CID
					}
				}
				// TODO :make sure book for this key exists ...
				tp.book[k].Positions[p.ID] = TradePosition{
					Position: model.TrackedPosition{
						Open:     oldPosition.Position.Open,
						Position: p,
					},
					Config: oldPosition.Config,
				}
			}
		}
		if !exists {
			// we need to add a new uncorrelated position
			// if p.CID == "" should have happened already in the previous step
			cfg := tp.getConfiguration(p.Coin)
			k, err := model.NewKeyFromString(p.CID)
			if err != nil {
				k = model.Key{}
				log.Warn().Str("cid", p.CID).Err(err).Msg("could not correlate position")
			}
			if _, ok := tp.book[k]; !ok {
				// load the portfolio if its there ...
				portfolio := Portfolio{
					Positions: make(map[string]TradePosition),
				}
				err := tp.state.Load(processor.NewStateKey(ProcessorName, k), &portfolio)
				log.Warn().Err(err).Msg("loaded portfolio")
				tp.book[k] = portfolio
			}
			tp.book[k].Positions[p.ID] = TradePosition{
				Position: model.TrackedPosition{
					Open:     p.OpenTime,
					Position: p,
				},
				Config: processor.GetConfig(cfg, k),
			}
		}
		// add the position id to make sure we remove any positions that were 'magically' closed
		posIDs[p.ID] = struct{}{}
	}
	// remove old book
	book := make(map[model.Key]Portfolio)
	for k, portfolio := range tp.book {
		positions := make(map[string]TradePosition, 0)
		for id, position := range portfolio.Positions {
			if _, ok := posIDs[id]; ok {
				positions[id] = position
			}
		}
		portfolio.Positions = positions
		book[k] = portfolio
		storage.Store(tp.state, processor.NewStateKey(ProcessorName, k), tp.book[k])
	}
	tp.book = book
	// save the state
	return nil
}

func (tp *tradePositions) getConfiguration(coin model.Coin) map[time.Duration]processor.Config {
	if cfg, ok := tp.initialConfigs[coin]; ok {
		return cfg
	}
	return map[time.Duration]processor.Config{}
}

func (tp *tradePositions) getAll() map[model.Key]map[string]TradePosition {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	positions := make(map[model.Key]map[string]TradePosition)
	for k, portfolio := range tp.book {
		positions[k] = make(map[string]TradePosition)
		for id, p := range portfolio.Positions {
			positions[k][id] = p
		}
	}
	return positions
}

func (tp *tradePositions) delete(k model.Key, id string) {
	delete(tp.book[k].Positions, id)
	log.Debug().Str("coin", fmt.Sprintf("%+v", k)).Str("id", id).Msg("deleted Position")
}

func (tp *tradePositions) budget(k model.Key, net float64) {
	if portfolio, ok := tp.book[k]; ok {
		portfolio.Budget += net
		tp.book[k] = portfolio
	} else {
		log.Error().Str("coin", fmt.Sprintf("%+v", k)).Msg("portfolio not found")
	}
}

// TODO : fix this logic to handle more edge cases
func (tp *tradePositions) checkOpen(k model.Key, order model.TrackedOrder) (model.TrackedOrder, bool) {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	correlatedPositions := tp.get(k)
	log.Info().Int("found", len(correlatedPositions)).Msg("correlated book")
	if len(correlatedPositions) > 0 {
		var same int
		var opp int
		var negVolume float64
		for _, cpos := range correlatedPositions {
			if cpos.Position.Type == order.Type {
				// how many should we open for this one ?
				same++
			} else if cpos.Position.Type.Inv() == order.Type {
				// apparently we ll close this one
				opp++
				negVolume += cpos.Position.Volume
			}
		}

		if same > 0 && opp == 0 {
			budget := tp.book[k].Budget
			if budget < 0 {
				log.Info().
					Int("num", same).
					Float64("budget", budget).
					Msg("cancel open positions")
				return order, false
			}
		} else if opp > 0 && same == 0 {
			order.Volume = negVolume
			log.Info().
				Int("num", opp).
				Float64("volume", negVolume).
				Msg("closing positions")
			return order, true
		} else {
			log.Info().
				Int("same", same).
				Int("opposite", opp).
				Msg("inconsistent correlated Positions")
			// TODO : we need to do some cleanup
		}
	}
	return order, true
}

func (tp *tradePositions) checkClose(trade *model.Trade) []tradeAction {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	actions := make([]tradeAction, 0)
	if !trade.Live {
		// only act on 'live' trades
		return actions
	}
	for k, portfolio := range tp.book {
		if k.Coin != trade.Coin {
			continue
		}
		for id, p := range portfolio.Positions {
			p.Position.CurrentPrice = trade.Price
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
				key:      k,
				position: p,
				doClose:  p.DoClose(),
			})
		}
	}
	for _, action := range actions {
		tp.book[action.key].Positions[action.position.Position.ID] = action.position
		storage.Store(tp.state, processor.NewStateKey(ProcessorName, action.key), tp.book[action.key])
	}
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

func (tp *tradePositions) close(client api.Exchange, user api.User, key model.Key, id string, time time.Time) bool {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	// ok, here we might encounter the case that we closed the Position from another trigger
	if _, ok := tp.book[key]; !ok {
		return false
	}
	if _, ok := tp.book[key].Positions[id]; !ok {
		return false
	}
	position := tp.book[key].Positions[id].Position
	net, profit := position.Value()
	err := client.ClosePosition(position.Position)
	position.Close = time
	if err != nil {
		log.Error().
			Err(err).
			Float64("volume", position.Volume).
			Str("id", id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("could not close Position")
		user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, "error")).
			AddLine(fmt.Sprintf("could not close %s [%s]: %s",
				key.ToString(),
				id,
				err.Error(),
			)).
			ReferenceTime(time), nil)
		return false
	} else {
		storage.Add(tp.registry, closeKey(position.Coin), position)
		log.Info().
			Float64("volume", position.Volume).
			Str("type", position.Type.String()).
			Str("id", id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("closed Position")
		// TODO : would be good if we do all together atomically
		tp.delete(key, id)
		tp.budget(key, net)
		storage.Store(tp.state, processor.NewStateKey(ProcessorName, key), tp.book[key])
		user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, "")).
			AddLine(fmt.Sprintf("%s closed %s %s ( %.3f | %.2f%s )",
				emoji.MapToSign(profit),
				key.ToString(),
				emoji.MapType(position.Type),
				position.Volume,
				profit,
				"%")).
			ReferenceTime(time), nil)
		return true
	}
}
