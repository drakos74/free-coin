package position

import (
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	NoPositionMsg = "no open positions"

	positionRefreshInterval = 5 * time.Minute
	ProcessorName           = "position"
)

var positionKey = api.ConsumerKey{
	Key:    "position",
	Prefix: "?p",
}

type tradeAction struct {
	key      tpKey
	position *tradePosition
	doClose  bool
}

// PositionTracker is the processor responsible for tracking open positions and acting on previous triggers.
// client is the exchange client used for closing positions
// user is the under interface for interacting with the user
// block is the internal synchronisation mechanism used to report on the process of requests
// closeInstant defines if the positions should be closed immediately of give the user the opportunity to act on them
func Position(client api.Exchange, user api.User, block api.Block, closeInstant bool, configs ...Config) api.Processor {
	// define our internal global statsCollector
	positions := tradePositions{
		pos:     make(map[model.Coin]map[string]*tradePosition),
		configs: configs,
		lock:    new(sync.RWMutex),
	}

	ticker := time.NewTicker(positionRefreshInterval)
	quit := make(chan struct{})
	go positions.track(client, ticker, quit, block)

	go positions.trackUserActions(client, user)

	err := positions.update(client)
	if err != nil {
		log.Error().Err(err).Msg("could not get initial positions")
	}

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {

		defer func() {
			log.Info().Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for trade := range in {
			// check on the existing positions
			// ignore if it s not the closing trade of the batch
			if !trade.Active {
				out <- trade
				continue
			}

			// TODO : integrate the above results to the 'Live' parameter
			for _, positionAction := range positions.checkClose(trade) {
				if positionAction.doClose {
					if closeInstant {
						positions.close(client, user, positionAction.key, trade.Time)
					} else {
						net, profit := positionAction.position.position.Value()
						msg := fmt.Sprintf("%s %s:%s (%s)",
							emoji.MapToSign(net),
							string(trade.Coin),
							coinmath.Format(profit),
							coinmath.Format(net))
						user.Send(api.Private, api.NewMessage(msg).ReferenceTime(trade.Time), &api.Trigger{
							ID:      positionAction.position.position.ID,
							Key:     positionKey,
							Default: []string{"?p", string(positionAction.key.coin), "close", positionAction.key.id},
							// TODO : instead of a big timeout check again when we want to close how the position is doing ...
						})
					}
				}
			}

			out <- trade
		}
	}
}
