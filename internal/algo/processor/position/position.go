package position

import (
	"context"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/metrics"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	trackingInterval = 30
)

var trackingConfigs = make([]model.TrackingConfig, 0)

func init() {
	trackingConfigs = []model.TrackingConfig{
		{
			Duration: 5 * time.Minute,
			Samples:  12,
		},
		{
			Duration: 15 * time.Minute,
			Samples:  16,
		},
	}
}

type tracker struct {
	index     api.Index
	user      api.User
	exchange  api.Exchange
	positions map[model.Coin]map[string]*model.Position
	lock      *sync.RWMutex
	state     string
}

func newTracker(index api.Index, u api.User, e api.Exchange) *tracker {
	t := &tracker{
		index:     index,
		user:      u,
		exchange:  e,
		positions: make(map[model.Coin]map[string]*model.Position),
		lock:      new(sync.RWMutex),
	}

	go func(t *tracker) {
		ticker := time.NewTicker(trackingInterval * time.Second)
		quit := make(chan struct{})
		for {
			select {
			case <-ticker.C:
				t.track()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(t)
	return t
}

func (t *tracker) track() {
	positions, err := t.exchange.OpenPositions(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("could not get positions")
		return
	}
	metrics.Observer.IncrementCalls("track", Name)
	pp, send := t.update(positions.Positions)
	// TODO : add trigger
	if len(pp) > 0 && send {
		msg, st, sh := formatPositions(pp)
		if st != t.state || sh {
			t.state = st
			t.user.Send(t.index, api.NewMessage(msg), nil)
		}
	}
}

func (t *tracker) update(positions []model.Position) (map[model.Coin][]position, bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	pp := make(map[model.Coin][]position, 0)
	hasUpdate := false
	for _, ps := range positions {
		if _, ok := t.positions[ps.Coin]; !ok {
			t.positions[ps.Coin] = make(map[string]*model.Position)
		}
		if _, ok := t.positions[ps.Coin][ps.ID]; !ok {
			ps.Profit = make(map[time.Duration]*model.Profit)
			for _, cfg := range trackingConfigs {
				ps.Profit[cfg.Duration] = model.NewProfit(&cfg)
			}
			// replace the positions ...
			func(ps model.Position) {
				t.positions[ps.Coin][ps.ID] = &ps
			}(ps)
		}

		// TODO : inject this logic into the position creation part
		posTime := ps.CurrentTime
		if posTime.IsZero() {
			posTime = time.Now()
		}

		// let the position digest the stats ...
		// TODO : probbly we dont nee to redo the polynomial work for the same coins ... e.g. 'aa'
		net, profit, aa := t.positions[ps.Coin][ps.ID].Value(model.NewPrice(ps.CurrentPrice, posTime))
		// only print if we were able to gather previous data, otherwise nothing will have changed
		if aa != nil {
			cc := make(map[time.Duration]float64)
			for d, a := range aa {
				cc[d] = a[2]
				// TODO : not the best way ... but anyway ...
				hasUpdate = true
			}
			if _, ok := pp[ps.Coin]; !ok {
				pp[ps.Coin] = make([]position, 0)
			}
			pp[ps.Coin] = append(pp[ps.Coin], position{
				t:       t.positions[ps.Coin][ps.ID].OpenTime,
				coin:    t.positions[ps.Coin][ps.ID].Coin,
				open:    t.positions[ps.Coin][ps.ID].OpenPrice,
				current: t.positions[ps.Coin][ps.ID].CurrentPrice,
				value:   net,
				diff:    profit,
				ratio:   cc,
			})
		}

		//if len(stats) > 0 {
		//	switch x := stats[len(stats)-1].(type) {
		//	case []buffer.TimeWindowView:
		//		// get the first, because we only asked for the first in the statsWindow args
		//		w := x[0]
		//		// TODO : define these conditions in the config
		//		fmt.Printf("w = %+v\n", w)
		//		if math.Abs(w.Ratio) > 0.01 {
		//			// track this position
		//
		//		}
		//	default:
		//		log.Error().Str("type", fmt.Sprintf(": %T\n", x)).Msg("Unsupported type for window")
		//	}
		//}
	}
	return pp, hasUpdate
}
