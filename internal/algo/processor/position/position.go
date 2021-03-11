package position

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	NoPositionMsg = "no open book"

	positionRefreshInterval = 5 * time.Minute
	ProcessorName           = "position"
)

var positionKey = api.ConsumerKey{
	Key:    "Position",
	Prefix: "?p",
}

type tradeAction struct {
	key      model.Key
	position TradePosition
	doClose  bool
}

func (tp *tradePositions) trackUserActions(client api.Exchange, user api.User) {
	for command := range user.Listen(positionKey.Key, positionKey.Prefix) {
		var action string
		var coin string
		var param string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?p", "?book", "?book"),
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
			api.Reply(api.Private, user, api.NewMessage(processor.Audit(ProcessorName, "error")).ReplyTo(command.ID), err)
			continue
		}

		switch action {
		// TODO : the below case wont work just right ... we need to send the loop-back trigger as in the initial close
		case "close":
		case "":
			err = tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get book")
				api.Reply(api.Private, user, api.NewMessage(processor.Audit(ProcessorName, "api error")).ReplyTo(command.ID), err)
			}
			i := 0
			if len(tp.book) == 0 {
				user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, NoPositionMsg)), nil)
				continue
			}
			for k, pos := range tp.getAll() {
				if c == "" || k.Coin == c {
					for id, p := range pos {
						net, profit := p.Position.Value()
						configMsg := fmt.Sprintf("[ profit : %.2f (%.2f) , stop-loss : %.2f (%.2f) ]",
							p.Config.Close.Profit.Min,
							p.Config.Close.Profit.High,
							p.Config.Close.Loss.Min,
							p.Config.Close.Loss.High,
						)
						msg := fmt.Sprintf("%s %s:%.2f%s(%.2f€) <- %s | %s",
							emoji.MapToSign(net),
							p.Position.Coin,
							profit,
							"%",
							net,
							emoji.MapType(p.Position.Type),
							coinmath.Format(p.Position.Volume),
						)
						// TODO : send a trigger for each Position to give access to adjust it
						trigger := &api.Trigger{
							ID:  id,
							Key: positionKey,
						}
						user.Send(api.Private, api.NewMessage(msg).AddLine(configMsg), trigger)
						i++
					}
				}
			}
		default:
			user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, "unknown command")).AddLine("[ close ]"), nil)
		}
	}
}

// TODO : create Position processor triggered only from trad actions and no Position tracking except for stop-loss
// TODO : need to take care of closing book
// PositionTracker is the processor responsible for tracking open book and acting on previous triggers.
// client is the exchange client used for closing book
// user is the under interface for interacting with the user
// block is the internal synchronisation mechanism used to report on the process of requests
// closeInstant defines if the book should be closed immediately of give the user the opportunity to act on them
func Position(shard storage.Shard, registry storage.Registry, client api.Exchange, user api.User, block api.Block, configs map[model.Coin]map[time.Duration]processor.Config) api.Processor {
	// define our internal global statsCollector
	positions := newPositionTracker(shard, registry, configs)

	ticker := time.NewTicker(positionRefreshInterval)
	quit := make(chan struct{})
	go positions.track(client, user, ticker, quit, block)

	go positions.trackUserActions(client, user)

	err := positions.update(client)
	if err != nil {
		log.Error().Err(err).Msg("could not get initial book")
	}

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {

		defer func() {
			log.Info().Str("processor", ProcessorName).Msg("closing processor")
			close(out)
		}()

		for trade := range in {
			// check on the existing book
			// ignore if it s not the closing trade of the batch
			if !trade.Active {
				out <- trade
				continue
			}

			// TODO : integrate the above results to the 'Live' parameter
			for _, positionAction := range positions.checkClose(trade) {
				if positionAction.doClose {
					if positionAction.position.Config.Close.Instant {
						positions.close(client, user, positionAction.key, positionAction.position.Position.ID, trade.Time)
					} else {
						net, profit := positionAction.position.Position.Value()
						msg := fmt.Sprintf("%s %s:%s (%s)",
							emoji.MapToSign(net),
							string(trade.Coin),
							coinmath.Format(profit),
							coinmath.Format(net))
						user.Send(api.Private, api.NewMessage(processor.Audit(ProcessorName, "alert")).
							AddLine(msg).
							ReferenceTime(trade.Time), &api.Trigger{
							ID:      positionAction.position.Position.ID,
							Key:     positionKey,
							Default: []string{"?p", positionAction.key.ToString(), "close", positionAction.position.Position.ID},
							// TODO : instead of a big timeout check again when we want to close how the Position is doing ...
						})
					}
				}
			}

			out <- trade
		}
	}
}
