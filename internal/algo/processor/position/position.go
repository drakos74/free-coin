package position

import (
	"fmt"
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

func (tp *tradePositions) trackUserActions(client api.Exchange, user api.User) {
	for command := range user.Listen(positionKey.Key, positionKey.Prefix) {
		var action string
		var coin string
		var param string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?p", "?pos", "?positions"),
			api.Any(&coin),
			// TODO : implement extend / reverse etc ...
			api.OneOf(&action, "buy", "sell", "close", ""),
			api.Any(&param),
		)
		c := model.Coin(coin)
		log.Debug().
			Str("command", command.Content).
			Str("action", action).
			Str("coin-argument", coin).
			Str("coin", string(c)).
			Str("param", param).
			Err(err).
			Msg("received user action")
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage(fmt.Sprintf("[%s cmd error]", ProcessorName)).ReplyTo(command.ID), err)
			continue
		}

		switch action {
		case "close":
			k := key(c, param)
			// close the damn position ...
			tp.close(client, user, k, time.Time{})
			// TODO : the below case wont work just right ... we need to send the loop-back trigger as in the initial close
		case "":
			err = tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
				api.Reply(api.Private, user, api.NewMessage("[api error]").ReplyTo(command.ID), err)
			}
			i := 0
			if len(tp.pos) == 0 {
				user.Send(api.Private, api.NewMessage(NoPositionMsg), nil)
				continue
			}
			for coin, pos := range tp.getAll() {
				if c == "" || coin == c {
					for id, p := range pos {
						net, profit := p.position.Value()
						configMsg := fmt.Sprintf("[ profit : %.2f (%.2f) , stop-loss : %.2f (%.2f) ]", p.config.Profit.Min, p.config.Profit.High, p.config.Loss.Min, p.config.Loss.High)
						msg := fmt.Sprintf("%s %s:%.2f%s(%.2f€) <- %s | %s",
							emoji.MapToSign(net),
							p.position.Coin,
							profit,
							"%",
							net,
							emoji.MapType(p.position.Type),
							coinmath.Format(p.position.Volume))
						// TODO : send a trigger for each position to give access to adjust it
						trigger := &api.Trigger{
							ID:  id,
							Key: positionKey,
						}
						user.Send(api.Private, api.NewMessage(msg).AddLine(configMsg), trigger)
						i++
					}
				}
			}
		}
	}
}

// PositionTracker is the processor responsible for tracking open positions and acting on previous triggers.
// client is the exchange client used for closing positions
// user is the under interface for interacting with the user
// block is the internal synchronisation mechanism used to report on the process of requests
// closeInstant defines if the positions should be closed immediately of give the user the opportunity to act on them
func Position(client api.Exchange, user api.User, block api.Block, closeInstant bool, configs ...Config) api.Processor {

	if len(configs) == 0 {
		configs = loadDefaults()
	}

	// define our internal global statsCollector
	positions := newPositionTracker(configs)

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
