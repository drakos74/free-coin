package position

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	trackingDuration = 5 * time.Minute
	trackingSamples  = 10
	trackingInterval = 30
)

type tracker struct {
	index     api.Index
	user      api.User
	exchange  api.Exchange
	positions map[string]*model.Position
	lock      *sync.RWMutex
}

func newTracker(index api.Index, u api.User, e api.Exchange) *tracker {
	t := &tracker{
		index:     index,
		user:      u,
		exchange:  e,
		positions: make(map[string]*model.Position),
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
	pp, send := t.update(positions.Positions)
	// TODO : add trigger
	if len(pp) > 0 && send {
		t.user.Send(t.index, api.NewMessage(formatPositions(pp)), nil)
	}
}

func (t *tracker) update(positions []model.Position) ([]position, bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	pp := make([]position, 0)
	hasUpdate := false
	for _, ps := range positions {
		if _, ok := t.positions[ps.ID]; !ok {
			ps.Profit = model.NewProfit(model.Track(trackingDuration, trackingSamples))
			// replace the positions ...
			func(ps model.Position) {
				t.positions[ps.ID] = &ps
			}(ps)
		}

		// TODO : inject this logic into the position creation part
		posTime := ps.CurrentTime
		if posTime.IsZero() {
			posTime = time.Now()
		}

		// let the position digest the stats ...
		net, profit, cc := t.positions[ps.ID].Value(model.NewPrice(ps.CurrentPrice, posTime))
		c := 0.0
		// only print if we were able to gather previous data, otherwise nothing will have changed
		if cc != nil {
			c = cc[2]
			if math.Abs(c) >= 0.001 {
				hasUpdate = true
			}
			pp = append(pp, position{
				t:       t.positions[ps.ID].OpenTime,
				coin:    t.positions[ps.ID].Coin,
				open:    t.positions[ps.ID].OpenPrice,
				current: t.positions[ps.ID].CurrentPrice,
				value:   net,
				diff:    profit,
				ratio:   c,
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
