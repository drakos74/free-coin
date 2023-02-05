package trader

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/drakos74/free-coin/internal/emoji"

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
	Value  float64
	Num    int
	Loss   int
	Profit int
}

type TrendReport struct {
	Profit           float64
	StopLossActive   bool
	TakeProfitActive bool
	ValidTrend       []model.Type
}

func newTracker() *tracker {
	return &tracker{
		PnLPerCoin:    make(map[model.Coin]Stats),
		PnLPerNetwork: make(map[string]Stats),
	}
}

func (t *tracker) add(coin model.Coin, network string, value float64, pnl float64) (coinPnl, globalPnl Stats) {
	tt := t.PnLPerCoin[coin]
	nn := t.PnLPerNetwork[network]
	if value > 0 {
		tt.Profit += 1
		tt.Value += value
		tt.PnL += pnl
		tt.Num += 1
		nn.Profit += 1
		nn.Value += value
		nn.PnL += pnl
		nn.Num += 1
		t.Stats.PnL += pnl
		t.Stats.Value += value
		t.Stats.Num += 1
		t.Stats.Profit += 1
	} else if value < 0 {
		tt.Loss += 1
		tt.PnL += pnl
		tt.Value += value
		tt.Num += 1
		nn.Loss += 1
		nn.PnL += pnl
		nn.Value += value
		nn.Num += 1
		t.Stats.PnL += pnl
		t.Stats.Value += value
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
	user     api.User
}

// SimpleTrader is a simple exchange trader
func SimpleTrader(id string, shard storage.Shard, registry storage.EventRegistry, settings Settings, e api.Exchange, u api.User) (*ExchangeTrader, error) {
	t, err := newTrader(id, shard, settings.TrackingConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create trader: %w", err)
	}
	eventRegistry, err := registry(EventRegistryPath)
	return NewExchangeTrader(t, e, eventRegistry, settings, u), nil
}

// NewExchangeTrader creates a new trading logic processor.
func NewExchangeTrader(trader *trader, exchange api.Exchange, registry storage.Registry, settings Settings, u api.User) *ExchangeTrader {
	exTrader := &ExchangeTrader{
		exchange: exchange,
		trader:   trader,
		settings: settings,
		tracker:  newTracker(),
		log:      NewEventLog(registry),
		user:     u,
	}
	exTrader.sync()
	return exTrader
}

func (xt *ExchangeTrader) sync() {
	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				pp, err := xt.UpstreamPositions(context.Background())
				if err != nil {
					if xt.user != nil {
						xt.user.Send(api.Index(xt.trader.account), api.NewMessage(fmt.Sprintf("err-get-pos ... %s", err.Error())), nil)
					}
					continue
				}
				_, positions := xt.CurrentPositions(model.AllCoins)
				for _, xp := range pp {
					found := false
					for k, pos := range positions {
						if xp.Coin == pos.Coin {
							found = true
							newPos, update := pos.Sync(xp)
							if update {
								xt.trader.positions[k] = newPos
								err = xt.trader.save()
								if xt.user != nil {
									xt.user.Send(api.Index(xt.trader.account), api.NewMessage(fmt.Sprintf(" synced with upstream %s | %v", formatPos(newPos), err)), nil)
								}
							}
						}
					}
					if !found {
						// open new position
						xp.Live = true
						xt.trader.positions[model.Key{Coin: xp.Coin}] = xp
						err = xt.trader.save()
						if xt.user != nil {
							xt.user.Send(api.Index(xt.trader.account), api.NewMessage(fmt.Sprintf(" synced with upstream %s | %v", formatPos(xp), err)), nil)
						}
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func formatPos(pos model.Position) string {
	return fmt.Sprintf("%s %s at %.2f * %.2f",
		pos.Coin, emoji.MapType(pos.Type), pos.OpenPrice, pos.Volume)
}

func (xt *ExchangeTrader) Stats() (map[model.Coin]Stats, map[string]Stats) {
	return xt.tracker.PnLPerCoin, xt.tracker.PnLPerNetwork
}

func (xt *ExchangeTrader) Settings() Settings {
	return xt.settings
}

func (xt *ExchangeTrader) OpenValue(openValue float64) Settings {
	xt.settings.OpenValue = openValue
	return xt.settings
}

func (xt *ExchangeTrader) StopLoss(stopLoss float64) Settings {
	xt.settings.StopLoss = stopLoss
	return xt.settings
}

func (xt *ExchangeTrader) TakeProfit(takeProfit float64) Settings {
	xt.settings.TakeProfit = takeProfit
	return xt.settings
}

// CurrentPositions returns all currently open positions
func (xt *ExchangeTrader) CurrentPositions(coins ...model.Coin) ([]model.Key, map[model.Key]model.Position) {
	return xt.trader.getAll(coins...)
}

// UpstreamPositions returns all currently open positions on the exchange
func (xt *ExchangeTrader) UpstreamPositions(ctx context.Context) ([]model.Position, error) {
	pp, err := xt.exchange.OpenPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not gtrader upstream positions: %w", err)
	}
	positions := make([]model.Position, 0)
	for _, p := range pp.Positions {
		positions = append(positions, p)
	}
	return positions, nil
}

// Update updates the positions and returns the ones over the stop Loss and take Profit thresholds
func (xt *ExchangeTrader) Update(trace map[string]bool, trade *model.TradeSignal, cfg []*model.TrackingConfig) (map[model.Key]model.Position, []float64, map[model.Key]map[time.Duration]model.Trend, map[model.Key]TrendReport) {
	pp := xt.trader.update(trace, trade, cfg)

	if xt.settings.TakeProfit == 0.0 {
		xt.settings.TakeProfit = math.MaxFloat64
	}
	if xt.settings.StopLoss == 0.0 {
		xt.settings.StopLoss = math.MaxFloat64
	}

	positions := make(map[model.Key]model.Position)

	allProfit := make([]float64, 0)

	allTrend := make(map[model.Key]map[time.Duration]model.Trend)

	reports := make(map[model.Key]TrendReport, 0)

	if len(pp) > 0 {
		for k, position := range pp {
			profit := position.PnL
			stopLossActivated := position.PnL <= -1*xt.settings.StopLoss
			takeProfitActivated := position.PnL >= xt.settings.TakeProfit
			report := TrendReport{
				Profit:           profit,
				StopLossActive:   stopLossActivated,
				TakeProfitActive: takeProfitActivated,
			}

			//shift := position.Trend.Shift != model.NoType
			//validShift := position.Trend.Shift != position.Type
			// TODO : NOT multi-position ready !!!
			// we assume it s only one position thats relevant
			for tt, trend := range position.Trend {
				// NOTE : Type here does not mean market , but Profit/Loss
				if validTrend, ok := trend.Assess(); ok {
					if _, ok := allTrend[k]; !ok {
						allTrend[k] = make(map[time.Duration]model.Trend)
					}
					allTrend[k][tt] = trend
					report.ValidTrend = validTrend
				}
			}
			//if stopLossActivated {
			//	// if we pass the stop-Loss threshold
			//	positions[k] = position
			//	delete(xt.Profit, k)
			//} else
			//if shift && validShift {
			//	// if there is a shift in the opposite direction of the position
			//	positions[k] = position
			//	delete(xt.Profit, k)
			//} else
			//fmt.Printf("[ valid = %v  , take-Profit = %v, stop-Loss = %v : %+v ]\n", validTrend, takeProfitActivated, stopLossActivated, Profit)
			if stopLossActivated || takeProfitActivated {
				if (report.ValidTrend[0] == model.Sell) || (report.ValidTrend[1] == model.Sell) {
					positions[k] = position
				}
			}
			reports[k] = report
			allProfit = append(allProfit, profit)
		}
	}
	return positions, allProfit, allTrend, reports
}

func (xt *ExchangeTrader) CreateOrder(key model.Key, time time.Time, price float64,
	openType model.Type, open bool, volume float64, reason Reason, live bool) (*model.TrackedOrder, bool, Event, error) {
	if volume == 0 {
		volume = xt.settings.OpenValue / price
	}

	close := ""
	// check the positions ...
	t := openType
	position, ok, positions := xt.trader.check(key)
	action := Event{
		Time:   time,
		Type:   openType,
		Price:  price,
		Key:    key,
		Reason: reason,
	}
	if ok {
		// we found a position to close
		volume = position.Volume
		action.Value = position.Value
		action.PnL = position.PnL
		// if we had a position already ...
		// TODO :review this ...
		if position.Type == openType {
			// we don't want to extend the current one ...
			log.Debug().
				Str("position", fmt.Sprintf("%+v", position)).
				Msg("ignoring signal")
			action.Reason = VoidReasonIgnore
			xt.log.append(action)
			action = xt.track(key, action)
			return nil, false, action, nil
		}
		// we need to close the position
		close = position.OrderID
		// by overriding the type it allows us to close and open a new one directly
		t = position.Type.Inv()
		log.Debug().
			Str("position", fmt.Sprintf("%+volume", position)).
			Str("type", t.String()).
			Float64("volume", volume).
			Msg("closing position")
	} else if len(positions) > 0 {
		// we did not find a position for this strategy but we found some for the same coin
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
				value += p.Value
			}
		}
		action.Value = value
		action.PnL = pnl
		if ignore {
			log.Debug().
				Str("positions", fmt.Sprintf("%+volume", positions)).
				Msg("ignoring conflicting signal")
			action.Reason = VoidReasonConflict
			xt.log.append(action)
			action = xt.track(key, action)
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
		xt.log.append(action)
		action = xt.track(key, action)
		return nil, false, action, fmt.Errorf("no clean type [%s %s:%v]", openType.String(), position.Type.String(), ok)
	}
	if close == "" {
		if !open {
			action.Reason = VoidReasonClose
			xt.log.append(action)
			action = xt.track(key, action)
			// we intended to close the position , but we dont have anything to close
			return nil, false, action, fmt.Errorf("ignoring close signal '%s' no open position for '%v'", openType.String(), key)
		} else {
			xt.log.append(action)
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
			Network:  key.Network,
			Strategy: key.Strategy,
		}, time, fmt.Sprintf("%+v", action))
	order.RefID = close
	order.Price = price
	var err error = nil
	if live {
		order, _, err = xt.exchange.OpenOrder(order)
		if err != nil {
			return nil, false, action, fmt.Errorf("could not send initial order: %w", err)
		}
	}
	if close == "" {
		err = xt.trader.add(order, live)
	} else {
		err = xt.trader.close(key)
		// and ... open a new one ...
		if open {
			if live {
				_, _, err = xt.exchange.OpenOrder(order)
				if err != nil {
					return nil, false, action, fmt.Errorf("could not send reverse order: %w", err)
				}
			}
			err = xt.trader.add(order, live)
		}
	}
	if err != nil {
		log.Error().Err(err).Msg("could not store position")
	}
	xt.tracker.add(order.Coin, key.Network, action.Value, action.PnL)
	action = xt.track(order.Key, action)
	return order, true, action, err
}

func (xt *ExchangeTrader) track(key model.Key, action Event) Event {
	action.Coin = xt.tracker.PnLPerCoin[key.Coin]
	action.Global = xt.tracker.Stats
	action.TradeTracker.Network = xt.tracker.PnLPerNetwork[key.Network]
	return action
}

func (xt *ExchangeTrader) Reset(coins ...model.Coin) (int, error) {
	pp, err := xt.trader.reset(coins...)
	return len(pp), err
}

// Actions returns the exchange actions so far
func (xt *ExchangeTrader) Actions() map[model.Coin][]Event {
	return xt.log.Events
}
