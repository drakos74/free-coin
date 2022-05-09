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

type tracker struct {
	PnLPerCoin    map[model.Coin]Stats
	PnLPerNetwork map[string]Stats
	Stats         Stats
}

type Stats struct {
	PnL    float64
	Num    int
	Loss   int
	Profit int
}

func newTracker() *tracker {
	return &tracker{
		PnLPerCoin:    make(map[model.Coin]Stats),
		PnLPerNetwork: make(map[string]Stats),
	}
}

func (t *tracker) add(coin model.Coin, network string, value float64) (coinPnl, globalPnl Stats) {
	tt := t.PnLPerCoin[coin]
	nn := t.PnLPerNetwork[network]
	if value > 0 {
		tt.Profit += 1
		tt.PnL += value
		tt.Num += 1
		nn.Profit += 1
		nn.PnL += value
		nn.Num += 1
		t.Stats.PnL += value
		t.Stats.Num += 1
		t.Stats.Profit += 1
	} else if value < 0 {
		tt.Loss += 1
		tt.PnL += value
		tt.Num += 1
		nn.Loss += 1
		nn.PnL += value
		nn.Num += 1
		t.Stats.PnL += value
		t.Stats.Num += 1
		t.Stats.Loss += 1
	}
	t.PnLPerCoin[coin] = tt
	t.PnLPerNetwork[network] = nn
	return tt, t.Stats
}

// ExchangeTrader implements the main trading logic.
type ExchangeTrader struct {
	exchange api.Exchange
	trader   *trader
	settings Settings
	tracker  *tracker
	log      *Log
}

// SimpleTrader is a simple exchange trader
func SimpleTrader(id string, shard storage.Shard, registry storage.EventRegistry, settings Settings, e api.Exchange) (*ExchangeTrader, error) {
	t, err := newTrader(id, shard, settings.TrackingConfig)
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
		settings: settings,
		tracker:  newTracker(),
		log:      NewEventLog(registry),
	}
}

func (et *ExchangeTrader) Stats() (map[model.Coin]Stats, map[string]Stats) {
	return et.tracker.PnLPerCoin, et.tracker.PnLPerNetwork
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

// Update updates the positions and returns the ones over the stop Loss and take Profit thresholds
func (et *ExchangeTrader) Update(trade *model.TradeSignal) (map[model.Key]model.Position, []float64, map[model.Key]map[time.Duration]model.Trend) {
	pp := et.trader.update(trade)

	if et.settings.TakeProfit == 0.0 {
		et.settings.TakeProfit = math.MaxFloat64
	}
	if et.settings.StopLoss == 0.0 {
		et.settings.StopLoss = math.MaxFloat64
	}

	positions := make(map[model.Key]model.Position)

	allProfit := make([]float64, 0)

	allTrend := make(map[model.Key]map[time.Duration]model.Trend)

	if len(pp) > 0 {
		for k, position := range pp {
			profit := position.PnL
			stopLossActivated := position.PnL <= -1*et.settings.StopLoss
			takeProfitActivated := position.PnL >= et.settings.TakeProfit
			//shift := position.Trend.Shift != model.NoType
			//validShift := position.Trend.Shift != position.Type
			// TODO : we have a trend ... any ... for now
			validTrend := make([]model.Type, 2)
			for tt, trend := range position.Trend {
				var hasTrend bool
				// NOTE : Type here does not mean market , but Profit/Loss
				if trend.Type[0] != model.NoType {
					// valid-trend
					validTrend[0] = trend.Type[0]
					hasTrend = trend.Live
				}
				if trend.Type[1] != model.NoType {
					// valid-trend
					validTrend[1] = trend.Type[1]
					hasTrend = trend.Live
				}
				if hasTrend {
					if _, ok := allTrend[k]; !ok {
						allTrend[k] = make(map[time.Duration]model.Trend)
					}
					allTrend[k][tt] = trend
				}
			}
			//if stopLossActivated {
			//	// if we pass the stop-Loss threshold
			//	positions[k] = position
			//	delete(et.Profit, k)
			//} else
			//if shift && validShift {
			//	// if there is a shift in the opposite direction of the position
			//	positions[k] = position
			//	delete(et.Profit, k)
			//} else
			//fmt.Printf("[ valid = %v  , take-Profit = %v, stop-Loss = %v : %+v ]\n", validTrend, takeProfitActivated, stopLossActivated, Profit)
			if stopLossActivated || takeProfitActivated {
				if (validTrend[0] == model.Sell) || (validTrend[1] == model.Sell) {
					positions[k] = position
				}
			}
			allProfit = append(allProfit, profit)
		}
	}

	return positions, allProfit, allTrend

}

func (et *ExchangeTrader) CreateOrder(key model.Key, time time.Time, price float64,
	openType model.Type, open bool, volume float64, reason Reason, network string, live bool) (*model.TrackedOrder, bool, Event, error) {
	if volume == 0 {
		volume = et.settings.OpenValue / price
	}

	close := ""
	// check the positions ...
	t := openType
	position, ok, positions := et.trader.check(key)
	action := Event{
		Time:    time,
		Network: network,
		Type:    openType,
		Price:   price,
		Key:     key,
		Reason:  reason,
	}
	if ok {
		volume = position.Volume
		value := 0.0
		// find out how much Profit we re making
		switch position.Type {
		case model.Buy:
			value = (price - position.OpenPrice) * position.Volume
		case model.Sell:
			value = (position.OpenPrice - price) * position.Volume
		}
		action.Value = value
		action.PnL = position.PnL
		// if we had a position already ...
		// TODO :review this ...
		if position.Type == openType {
			// but .. we dont want to extend the current one ...
			log.Debug().
				Str("position", fmt.Sprintf("%+v", position)).
				Msg("ignoring signal")
			action.Reason = VoidReasonIgnore
			et.log.append(action)
			action = et.track(key.Coin, action)
			return nil, false, action, nil
		}
		// we need to close the position
		close = position.OrderID
		t = position.Type.Inv()
		// allows us to close and open a new one directly
		log.Debug().
			Str("position", fmt.Sprintf("%+volume", position)).
			Str("type", t.String()).
			Float64("volume", volume).
			Msg("closing position")
	} else if len(positions) > 0 {
		var ignore bool
		pnl := 0.0
		value := 0.0
		for _, p := range positions {
			if p.Type != openType {
				// if it will be an opposite opening to the current position
				log.Debug().
					Str("positions", fmt.Sprintf("%+volume", positions)).
					Msg("closing position")
			} else {
				pnl += p.PnL
				switch p.Type {
				case model.Buy:
					value += (price - position.OpenPrice) * position.Volume
				case model.Sell:
					value += (position.OpenPrice - price) * position.Volume
				}
			}
		}
		action.Value = value
		action.PnL = pnl
		if ignore {
			log.Debug().
				Str("positions", fmt.Sprintf("%+volume", positions)).
				Msg("ignoring conflicting signal")
			action.Reason = VoidReasonConflict
			et.log.append(action)
			action = et.track(key.Coin, action)
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
		action = et.track(key.Coin, action)
		return nil, false, action, fmt.Errorf("no clean type [%s %s:%v]", openType.String(), position.Type.String(), ok)
	}
	if close == "" {
		if !open {
			action.Reason = VoidReasonClose
			et.log.append(action)
			action = et.track(key.Coin, action)
			// we intended to close the position , but we dont have anything to close
			return nil, false, action, fmt.Errorf("ignoring close signal '%s' no open position for '%v'", openType.String(), key)
		} else {
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
	var err error = nil
	if live {
		order, _, err = et.exchange.OpenOrder(order)
		if err != nil {
			return nil, false, action, fmt.Errorf("could not send initial order: %w", err)
		}
	}
	if close == "" {
		err = et.trader.add(key, order, live, network)
	} else {
		err = et.trader.close(key)
		// and ... open a new one ...
		if open {
			if live {
				_, _, err = et.exchange.OpenOrder(order)
				if err != nil {
					return nil, false, action, fmt.Errorf("could not send reverse order: %w", err)
				}
			}
			err = et.trader.add(key, order, live, network)
		}
	}
	if err != nil {
		log.Error().Err(err).Msg("could not store position")
	}
	et.tracker.add(order.Coin, action.Network, action.Value)
	action = et.track(order.Coin, action)
	return order, true, action, err
}

func (et *ExchangeTrader) track(coin model.Coin, action Event) Event {
	action.Coin = et.tracker.PnLPerCoin[coin]
	action.Global = et.tracker.Stats
	action.TradeTracker.Network = et.tracker.PnLPerNetwork[action.Network]
	return action
}

func (et *ExchangeTrader) Reset(coins ...model.Coin) (int, error) {
	pp, err := et.trader.reset(coins...)
	return len(pp), err
}

// Actions returns the exchange actions so far
func (et *ExchangeTrader) Actions() map[model.Coin][]Event {
	return et.log.Events
}
