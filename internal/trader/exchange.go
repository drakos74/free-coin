package trader

import (
	"fmt"
	"math"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// ExchangeTrader implements the main trading logic.
type ExchangeTrader struct {
	exchange api.Exchange
	trader   *trader
	profit   map[model.Key]float64
	log      *Log
}

// SimpleTrader is a simple exchange trader
func SimpleTrader(id string, shard storage.Shard, settings map[model.Coin]map[time.Duration]Settings, e api.Exchange) (*ExchangeTrader, error) {
	t, err := newTrader(id, shard, settings)
	if err != nil {
		return nil, fmt.Errorf("could not create trader: %w", err)
	}
	return NewExchangeTrader(t, e), nil
}

// NewExchangeTrader creates a new trading logic processor.
func NewExchangeTrader(trader *trader, exchange api.Exchange) *ExchangeTrader {
	return &ExchangeTrader{
		exchange: exchange,
		trader:   trader,
		profit:   make(map[model.Key]float64),
		log:      NewEventLog(),
	}
}

// Positions returns all currently open positions
func (et *ExchangeTrader) Positions() ([]model.Key, map[model.Key]model.Position) {
	return et.trader.getAll(nil)
}

// CheckPosition checks the position status for the given key
func (et *ExchangeTrader) CheckPosition(key model.Key, price float64, tp, sl float64) (map[model.Key]model.Position, float64) {
	p, ok, pp := et.trader.check(key)

	if tp == 0.0 {
		tp = math.MaxFloat64
	}
	if sl == 0.0 {
		sl = math.MaxFloat64
	}

	positions := make(map[model.Key]model.Position)

	if ok {
		pp[key] = p
	}

	allProfit := 0.0

	if len(pp) > 0 {
		for k, position := range pp {
			profit := 0.0
			switch position.Type {
			case model.Buy:
				profit = (price - position.OpenPrice) / position.OpenPrice
			case model.Sell:
				profit = (position.OpenPrice - price) / position.OpenPrice
			}

			lastProfit := et.profit[k]
			if profit > 0 {
				if profit > tp && profit < lastProfit {
					positions[k] = position
					delete(et.profit, k)
				} else {
					et.profit[k] = profit
				}
			} else {
				if profit < -1*sl && profit > lastProfit {
					positions[k] = position
					delete(et.profit, k)
				} else {
					et.profit[k] = profit
				}
			}
			allProfit += profit
		}
	}

	return positions, allProfit

}

func (et *ExchangeTrader) CreateOrder(key model.Key, time time.Time, price float64,
	openType model.Type, open bool, volume float64) (*model.TrackedOrder, bool, Action, error) {
	close := ""
	// check the positions ...
	t := openType
	position, ok, positions := et.trader.check(key)
	action := Action{
		Time:  time,
		Type:  openType,
		Price: price,
		Key:   key,
	}
	if ok {
		// if we had a position already ...
		// TODO :review this ...
		if position.Type == openType {
			// but .. we dont want to extend the current one ...
			log.Debug().
				Str("position", fmt.Sprintf("%+v", position)).
				Msg("ignoring signal")
			action.Reason = VoidReasonIgnore
			et.log.append(action)
			return nil, false, action, nil
		}
		// we need to close the position
		close = position.OrderID
		t = position.Type.Inv()
		// allows us to close and open a new one directly
		volume = position.Volume
		value := 0.0
		reason := SignalReason
		// find out how much profit we re making
		switch position.Type {
		case model.Buy:
			value = (price - position.OpenPrice) / position.OpenPrice
		case model.Sell:
			value = (position.OpenPrice - price) / position.OpenPrice
		}
		if !open {
			if value > 0 {
				reason = TakeProfitReason
			} else {
				reason = StopLossReason
			}
		}
		log.Debug().
			Str("position", fmt.Sprintf("%+volume", position)).
			Str("type", t.String()).
			Float64("volume", volume).
			Msg("closing position")
		action.Value = value
		action.Reason = reason
		et.log.append(action)
	} else if len(positions) > 0 {
		var ignore bool
		for _, p := range positions {
			if p.Type != openType {
				// if it will be an opposite opening to the current position
				log.Debug().
					Str("positions", fmt.Sprintf("%+volume", positions)).
					Msg("closing position")
			}
		}
		if ignore {
			log.Debug().
				Str("positions", fmt.Sprintf("%+volume", positions)).
				Msg("ignoring conflicting signal")
			action.Reason = VoidReasonConflict
			et.log.append(action)
			return nil, false, action, nil
		}
		log.Debug().
			Str("position", fmt.Sprintf("%+v", position)).
			Str("type", t.String()).
			Str("open-type", openType.String()).
			Float64("volume", volume).
			Msg("opening position")
	}
	if t == 0 {
		action.Reason = VoidReasonType
		et.log.append(action)
		return nil, false, action, fmt.Errorf("no clean type [%s %s:%v]", openType.String(), position.Type.String(), ok)
	}
	if close == "" {
		if !open {
			action.Reason = VoidReasonClose
			et.log.append(action)
			// we intended to close the position , but we dont have anything to close
			return nil, false, action, fmt.Errorf("ignoring close signal '%s' no open position for '%v'", openType.String(), key)
		} else {
			action.Reason = SignalReason
			et.log.append(action)
		}
	}
	order := model.NewOrder(key.Coin).
		Market().
		WithType(t).
		WithVolume(volume).
		CreateTracked(model.Key{
			Coin:     key.Coin,
			Duration: key.Duration,
			Strategy: key.ToString(),
		}, time)
	order.RefID = close
	order.Price = price
	order, _, err := et.exchange.OpenOrder(order)
	if err != nil {
		return nil, false, action, fmt.Errorf("could not send initial order: %w", err)
	}
	if close == "" {
		err = et.trader.add(key, order)
	} else {
		err = et.trader.close(key)
		// and ... open a new one ...
		if open {
			_, _, err = et.exchange.OpenOrder(order)
			if err != nil {
				return nil, false, action, fmt.Errorf("could not send reverse order: %w", err)
			}
			err = et.trader.add(key, order)
		}
	}
	if err != nil {
		log.Error().Err(err).Msg("could not store position")
	}
	return order, true, action, err
}

// Actions returns the exchange actions so far
func (et *ExchangeTrader) Actions() map[model.Coin][]Action {
	return et.log.Actions
}
