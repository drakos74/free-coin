package trader

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type ExchangeTrader struct {
	exchange api.Exchange
	trader   *Trader
}

func NewExchangeTrader(trader *Trader, exchange api.Exchange) *ExchangeTrader {
	return &ExchangeTrader{
		exchange: exchange,
		trader:   trader,
	}
}

func (et *ExchangeTrader) CreateOrder(
	key string, time time.Time, price float64,
	duration time.Duration, coin model.Coin,
	openType, closeType model.Type, volume float64) (*model.TrackedOrder, float64, error) {
	close := ""
	// check the positions ...
	t := openType
	position, ok, positions := et.trader.check(key, coin)
	if ok {
		// if we had a position already ...
		// TODO :review this ...
		if position.Type == closeType {
			// but .. we dont want to extend the current one ...
			log.Debug().
				Str("position", fmt.Sprintf("%+volume", position)).
				Msg("ignoring signal")
			return nil, 0, fmt.Errorf("position already exists")
		}
		// we need to close the position
		close = position.OrderID
		t = position.Type.Inv()
		volume = position.Volume
		log.Debug().
			Str("position", fmt.Sprintf("%+volume", position)).
			Str("type", t.String()).
			Float64("volume", volume).
			Msg("closing position")
	} else if len(positions) > 0 {
		var ignore bool
		for _, p := range positions {
			if p.Type != openType {
				// if it will be an opposite opening to the current position,
				// it will act as a close, and it will break our metrics ...
				ignore = true
			}
		}
		if ignore {
			log.Debug().
				Str("positions", fmt.Sprintf("%+volume", positions)).
				Msg("ignoring conflicting signal")
			return nil, 0, fmt.Errorf("ignoring conflicting signal")
		}
		log.Debug().
			Str("position", fmt.Sprintf("%+volume", position)).
			Str("type", t.String()).
			Str("open-type", openType.String()).
			Str("close-type", closeType.String()).
			Float64("volume", volume).
			Msg("opening position")
	}
	if t == 0 {
		return nil, 0, fmt.Errorf("no clean type [%s %s]", openType.String(), closeType.String())
	}
	order := model.NewOrder(coin).
		Market().
		WithType(t).
		WithVolume(volume).
		CreateTracked(model.Key{
			Coin:     coin,
			Duration: duration,
			Strategy: key,
		}, time)
	order.RefID = close
	order.Price = price
	order, _, err := et.exchange.OpenOrder(order)
	if err != nil {
		return nil, 0, fmt.Errorf("could not send order: %w", err)
	}
	profit, err := et.trader.add(key, order, close)
	if err != nil {
		log.Error().Err(err).Msg("could not store position")
	}
	return order, profit, err
}
