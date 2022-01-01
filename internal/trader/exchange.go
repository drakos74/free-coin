package trader

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type ExchangeTrader struct {
	exchange api.Exchange
	trader   *trader
}

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
	}
}

func (et *ExchangeTrader) CreateOrder(key Key, time time.Time, price float64,
	openType model.Type, volume float64) (*model.TrackedOrder, bool, error) {
	close := ""
	// check the positions ...
	t := openType
	position, ok, positions := et.trader.check(key)

	if ok {
		// if we had a position already ...
		// TODO :review this ...
		if position.Type == openType {
			// but .. we dont want to extend the current one ...
			log.Debug().
				Str("position", fmt.Sprintf("%+volume", position)).
				Msg("ignoring signal")
			return nil, false, nil
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
			return nil, false, nil
		}
		log.Debug().
			Str("position", fmt.Sprintf("%+v", position)).
			Str("type", t.String()).
			Str("open-type", openType.String()).
			Float64("volume", volume).
			Msg("opening position")
	}
	if t == 0 {
		return nil, false, fmt.Errorf("no clean type [%s %s:%v]", openType.String(), position.Type.String(), ok)
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
		return nil, false, fmt.Errorf("could not send initial order: %w", err)
	}
	if close == "" {
		err = et.trader.add(key, order)
	} else {
		err = et.trader.close(key)
		// open a new one ...
		order, _, err = et.exchange.OpenOrder(order)
		if err != nil {
			return nil, false, fmt.Errorf("could not send reverse order: %w", err)
		}
		err = et.trader.add(key, order)
	}
	if err != nil {
		log.Error().Err(err).Msg("could not store position")
	}
	return order, true, err
}
