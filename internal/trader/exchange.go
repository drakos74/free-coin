package trader

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	EventRegistryPath = "events"
)

// ExchangeTrader implements the main trading logic.
type ExchangeTrader struct {
	exchange api.Exchange
	trader   *trader
	profit   map[model.Key]float64
	settings Settings
	log      *Log
}

// SimpleTrader is a simple exchange trader
func SimpleTrader(id string, shard storage.Shard, registry storage.EventRegistry, settings Settings, e api.Exchange) (*ExchangeTrader, error) {
	t, err := newTrader(id, shard)
	if err != nil {
		return nil, fmt.Errorf("could not create trader: %w", err)
	}
	eventRegistry, err := registry(EventRegistryPath)
	return NewExchangeTrader(t, e, eventRegistry, settings), nil
}

// NewExchangeTrader creates a new trading logic processor.
func NewExchangeTrader(trader *trader, exchange api.Exchange, registry storage.Registry, settings Settings) *ExchangeTrader {
	return &ExchangeTrader{
		exchange: exchange,
		trader:   trader,
		profit:   make(map[model.Key]float64),
		settings: settings,
		log:      NewEventLog(registry),
	}
}

func (et *ExchangeTrader) Settings() Settings {
	return et.settings
}

func (et *ExchangeTrader) OpenValue(openValue float64) Settings {
	et.settings.OpenValue = openValue
	return et.settings
}

func (et *ExchangeTrader) StopLoss(stopLoss float64) Settings {
	et.settings.StopLoss = stopLoss
	return et.settings
}

func (et *ExchangeTrader) TakeProfit(takeProfit float64) Settings {
	et.settings.TakeProfit = takeProfit
	return et.settings
}

// CurrentPositions returns all currently open positions
func (et *ExchangeTrader) CurrentPositions(coins ...model.Coin) ([]model.Key, map[model.Key]model.Position) {
	return et.trader.getAll(coins...)
}

// UpstreamPositions returns all currently open positions on the exchange
func (et *ExchangeTrader) UpstreamPositions(ctx context.Context) ([]model.Position, error) {
	pp, err := et.exchange.OpenPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get upstream positions: %w", err)
	}
	positions := make([]model.Position, 0)
	for _, p := range pp.Positions {
		positions = append(positions, p)
	}
	return positions, nil
}

// Update updates the positions and returns the ones over the stop loss and take profit thresholds
func (et *ExchangeTrader) Update(trade *model.Trade) (map[model.Key]model.Position, float64) {
	if trade.Meta.Num > 0 {
		trade = &model.Trade{
			Coin:   trade.Coin,
			Price:  trade.Meta.Price / float64(trade.Meta.Num),
			Volume: trade.Meta.Volume / float64(trade.Meta.Num),
			Time:   trade.Time,
		}
	}

	pp := et.trader.update(trade)

	if et.settings.TakeProfit == 0.0 {
		et.settings.TakeProfit = math.MaxFloat64
	}
	if et.settings.StopLoss == 0.0 {
		et.settings.StopLoss = math.MaxFloat64
	}

	positions := make(map[model.Key]model.Position)

	allProfit := 0.0

	if len(pp) > 0 {
		for k, position := range pp {
			profit := position.PnL
			//lastProfit := et.profit[k]
			if position.Trend.Type != model.NoType {
				if position.Trend.Type != position.Type {
					positions[k] = position
					delete(et.profit, k)
				}
			} else if position.Trend.Shift != model.NoType {
				if position.Trend.Shift != position.Type &&
					((position.PnL < -1*et.settings.StopLoss) ||
						(position.PnL > et.settings.TakeProfit)) {
					positions[k] = position
					delete(et.profit, k)
				}
			}
			//else if position.PnL > 0 {
			//	if profit > et.settings.TakeProfit {
			//		positions[k] = position
			//		delete(et.profit, k)
			//	} else {
			//		et.profit[k] = profit
			//	}
			//} else {
			//	// We dont want trailing back for loss
			//	if profit < -1*et.settings.StopLoss {
			//		positions[k] = position
			//		delete(et.profit, k)
			//	} else {
			//		et.profit[k] = profit
			//	}
			//}
			allProfit += profit
		}
	}

	return positions, allProfit

}

func (et *ExchangeTrader) CreateOrder(key model.Key, time time.Time, price float64,
	openType model.Type, open bool, volume float64) (*model.TrackedOrder, bool, Event, error) {

	if volume == 0 {
		volume = et.settings.OpenValue / price
	}

	close := ""
	// check the positions ...
	t := openType
	position, ok, positions := et.trader.check(key)
	action := Event{
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
			action.PnL = position.PnL
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
		action.PnL = position.PnL
		et.log.append(action)
	} else if len(positions) > 0 {
		var ignore bool
		pnl := 0.0
		for _, p := range positions {
			if p.Type != openType {
				// if it will be an opposite opening to the current position
				log.Debug().
					Str("positions", fmt.Sprintf("%+volume", positions)).
					Msg("closing position")
			} else {
				pnl += p.PnL
			}
		}
		if ignore {
			log.Debug().
				Str("positions", fmt.Sprintf("%+volume", positions)).
				Msg("ignoring conflicting signal")
			action.Reason = VoidReasonConflict
			action.PnL = pnl
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
		WithLeverage(model.L_5).
		CreateTracked(model.Key{
			Coin:     key.Coin,
			Duration: key.Duration,
			Strategy: key.ToString(),
		}, time, fmt.Sprintf("%+v", action))
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

func (et *ExchangeTrader) Reset(coins ...model.Coin) (int, error) {
	pp, err := et.trader.reset(coins...)
	return len(pp), err
}

// Actions returns the exchange actions so far
func (et *ExchangeTrader) Actions() map[model.Coin][]Event {
	return et.log.Events
}
