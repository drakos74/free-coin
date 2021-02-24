package processor

import (
	"context"
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
	positionRefreshInterval = 5 * time.Minute
	positionProcessorName   = "position"
)

var positionKey = api.ConsumerKey{
	Key:    "position",
	Prefix: "?p",
}

type tpKey struct {
	coin model.Coin
	id   string
}

func key(c model.Coin, id string) tpKey {
	return tpKey{
		coin: c,
		id:   id,
	}
}

type tradePositions struct {
	pos  map[model.Coin]map[string]*tradePosition
	lock *sync.RWMutex
}

func (tp tradePositions) checkClose(trade *model.Trade) (tpKey, model.Position, bool) {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	for c, positions := range tp.pos {
		if c == trade.Coin {
			for id, p := range positions {
				tp.pos[c][id].position.CurrentPrice = trade.Price
				if trade.Live {
					net, profit := p.position.Value()
					log.Debug().
						Str("coin", string(p.position.Coin)).
						Float64("net", net).
						Float64("profit", profit).
						Msg("check position")
					return tpKey{
						coin: c,
						id:   id,
					}, p.position, p.DoClose()
				}
			}
		}
	}
	return tpKey{}, model.Position{}, false
}

func (tp tradePositions) updateConfig(key tpKey, profit, stopLoss float64) {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	tp.pos[key.coin][key.id].config.profit = profit
	tp.pos[key.coin][key.id].config.stopLoss = stopLoss
}

func (tp tradePositions) update(client api.Exchange) error {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	pp, err := client.OpenPositions(context.Background())
	if err != nil {
		return fmt.Errorf("could not get positions: %w", err)
	}
	for _, p := range pp.Positions {
		cfg := getConfiguration(p.Coin)
		if p, ok := tp.pos[p.Coin][p.ID]; ok {
			cfg = p.config
		}
		if _, ok := tp.pos[p.Coin]; !ok {
			tp.pos[p.Coin] = make(map[string]*tradePosition)
		}
		tp.pos[p.Coin][p.ID] = &tradePosition{
			position: p,
			config:   cfg,
		}
	}
	return nil
}

func (tp tradePositions) track(client api.Exchange, ticker *time.Ticker, quit chan struct{}, update <-chan api.Action) {
	// and update the positions at the predefined interval.
	for {
		select {
		case <-update: // trigger update on the positions on external events/actions
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
			}
		case <-ticker.C: // trigger an update on the positions at regular intervals
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
			}
		case <-quit: // stop the tracking of positions
			ticker.Stop()
			return
		}
	}
}

const noPositionMsg = "no open positions"

func (tp tradePositions) trackUserActions(client api.Exchange, user api.User) {
	for command := range user.Listen(positionKey.Key, positionKey.Prefix) {
		var action string
		var coin string
		var param string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?p", "?pos", "?positions"),
			api.Any(&coin),
			api.OneOf(&action, "buy", "sell", "close", ""),
			api.Any(&param),
		)
		c := model.Coin(coin)

		log.Debug().
			Str("action", action).
			Str("coin-argument", coin).
			Str("coin", string(c)).
			Str("param", param).
			Err(err).
			Msg("received user action")
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage(fmt.Sprintf("[%s cmd error]", positionProcessorName)).ReplyTo(command.ID), err)
			continue
		}

		switch action {
		case "close":
			k := tpKey{
				coin: c,
				id:   param,
			}
			// close the damn position ...
			if position, ok := tp.get(k); ok {
				net, profit := position.Value()
				err := client.ClosePosition(position)
				if err != nil {
					log.Error().Float64("volume", position.Volume).Str("id", k.id).Str("coin", string(position.Coin)).Float64("net", net).Float64("profit", profit).Msg("could not close position")
					user.Send(api.Private, api.NewMessage(fmt.Sprintf("could not close %s [%s]: %s", k.coin, k.id, err.Error())), nil)
				} else {
					log.Info().Float64("volume", position.Volume).Str("id", k.id).Str("coin", string(position.Coin)).Float64("net", net).Float64("profit", profit).Msg("closed position")
					delete(tp.pos[k.coin], k.id)
					user.Send(api.Private, api.NewMessage(fmt.Sprintf("closed %s at %.2f [%s]", k.coin, profit, k.id)), nil)
				}
			}
			// TODO : the below case wont work just right ... we need to send the loop-back trigger as in the initial close
		case "":
			err = tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
				api.Reply(api.Private, user, api.NewMessage("[api error]").ReplyTo(command.ID), err)
			}
			i := 0
			if len(tp.pos) == 0 {
				user.Send(api.Private, api.NewMessage(noPositionMsg), nil)
				continue
			}
			for coin, pos := range tp.pos {
				if c == "" || coin == c {
					for id, p := range pos {
						net, profit := p.position.Value()
						profitThreshold := p.config.profit
						stopLossThreshold := p.config.stopLoss
						configMsg := fmt.Sprintf("[ profit : %.2f , stop-loss : %.2f ]", profitThreshold, stopLossThreshold)
						msg := fmt.Sprintf("%s %s:%.2f%s(%.2f§) <- %v at %v",
							emoji.MapToSign(net),
							p.position.Coin,
							profit,
							"%",
							net,
							p.position.Type,
							coinmath.Format(p.position.OpenPrice))
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

func (tp tradePositions) get(key tpKey) (model.Position, bool) {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	// ok, here we might encounter the case that we closed the position from another trigger
	if _, ok := tp.pos[key.coin]; !ok {
		return model.Position{}, false
	}
	if _, ok := tp.pos[key.coin][key.id]; !ok {
		return model.Position{}, false
	}
	return tp.pos[key.coin][key.id].position, true
}

func (tp tradePositions) exists(key tpKey) bool {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	// ok, here we might encounter the case that we closed the position from another trigger
	if _, ok := tp.pos[key.coin]; !ok {
		return false
	}
	if _, ok := tp.pos[key.coin][key.id]; !ok {
		return false
	}
	return true
}

// TODO : refactor the whole trigger functionality for the command to return back to the processor.
// TODO : Processor should have full control of it's actions and not delegate with closures !!!
func (tp tradePositions) closePositionTrigger(client api.Exchange, key tpKey) api.TriggerFunc {
	// add a flag to the position that we want to close it
	//if tp.exists(key) {
	//	blockClose := tp[key.coin][key.id].config.blockClose
	//	if blockClose {
	//		// position is already triggered for close ...
	//		return func(command api.Command) (string, error) {
	//			return "", fmt.Errorf("position already closing for key %+v", key)
	//		}
	//	}
	//}
	return func(command api.Command) (string, error) {
		fmt.Println(fmt.Sprintf("command = %+v", command))
		var nProfit float64
		var nStopLoss float64
		exec, err := command.Validate(
			api.AnyUser(),
			api.Contains("skip", "extend", "reverse", "close"),
			api.Float(&nProfit),
			api.Float(&nStopLoss),
		)
		if err != nil {
			return "", fmt.Errorf("invalid command: %w", err)
		}
		// ok, here we might encounter the case that we closed the position from another trigger
		if !tp.exists(key) {
			log.Error().Str("id", key.id).Str("coin", string(key.coin)).Msg("could not find previous open position")
			return "", fmt.Errorf("could not find previous open position")
		}
		// check if closing conditions still hold
		// TODO : make this only for the negative case
		if !tp.pos[key.coin][key.id].DoClose() {
			log.Error().Str("id", key.id).Str("coin", string(key.coin)).Msg("position conditions reversed")
			return "", fmt.Errorf("position conditions reversed")
		}
		position := tp.pos[key.coin][key.id].position
		net, profit := position.Value()
		switch exec {
		case "skip":
			// avoid to complete the trigger in any case
			return "no action", nil
		case "extend":
			if nProfit == 0 || nStopLoss == 0 {
				return "", fmt.Errorf("cannot adjust profit and loss: %v - %v", nProfit, nStopLoss)
			}
			tp.updateConfig(key, nProfit, nStopLoss)
			return fmt.Sprintf("%s position for %s [ profit = %f , stop-loss = %f]", exec, string(position.Coin), tp.pos[key.coin][key.id].config.profit, tp.pos[key.coin][key.id].config.stopLoss), nil
		case "reverse":
			position.Volume = 2 * position.Volume
			fallthrough
		case "close":
			// TODO : check also market conditions from enriched trades !!!
			err := client.ClosePosition(position)
			if err != nil {
				log.Error().Float64("volume", position.Volume).Str("id", key.id).Str("coin", string(position.Coin)).Float64("net", net).Float64("profit", profit).Msg("could not close position")
				return "", fmt.Errorf("could not complete command: %w", err)
			} else {
				log.Info().Float64("volume", position.Volume).Str("id", key.id).Str("coin", string(position.Coin)).Float64("net", net).Float64("profit", profit).Msg("closed position")
				delete(tp.pos[key.coin], key.id)
				return fmt.Sprintf("%s position for %s ( type : %v net : %v vol : %.2f )", exec, string(position.Coin), position.Type, coinmath.Format(net), position.Volume), nil
			}
		}
		return "", fmt.Errorf("only allowed commands are [ close , extend , reverse , skip ]")
	}
}

type tradePosition struct {
	position model.Position
	config   closingConfig
}

// DoClose checks if the given position should be closed, based on the current configuration.
// TODO : test this logic
func (tp *tradePosition) DoClose() bool {
	net, p := tp.position.Value()
	if net > 0 && p > tp.config.profit {
		// check the previous profit in order to extend profit
		if p > tp.config.highProfit {
			// if we are making more ... ignore
			tp.config.highProfit = p
			return false
		}
		diff := tp.config.highProfit - p
		if diff < 0.15 {
			// leave for now, hoping profit will go up again
			// but dont update our highest value
			return false
		}
		// only close if the market is going down
		return tp.config.Live <= 0
	}
	if net < 0 && p < -1*tp.config.stopLoss {
		if p > tp.config.lowLoss {
			tp.config.lowLoss = p
			// we are improving our position ... so give it a bit of time.
			return false
		}
		tp.config.lowLoss = p
		// only close if the market is going up
		return tp.config.Live >= 0
	}
	// check if we missed a profit opportunity here
	return false
}

// PositionTracker is the processor responsible for tracking open positions and acting on previous triggers.
func Position(client api.Exchange, user api.User, update <-chan api.Action) api.Processor {

	// define our internal global statsCollector
	positions := tradePositions{
		pos:  make(map[model.Coin]map[string]*tradePosition),
		lock: new(sync.RWMutex),
	}

	ticker := time.NewTicker(positionRefreshInterval)
	quit := make(chan struct{})
	go positions.track(client, ticker, quit, update)

	go positions.trackUserActions(client, user)

	err := positions.update(client)
	if err != nil {
		log.Error().Err(err).Msg("could not get initial positions")
	}

	return func(in <-chan *model.Trade, out chan<- *model.Trade) {

		defer func() {
			log.Info().Str("processor", positionProcessorName).Msg("closing' strategy")
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
			if k, position, ok := positions.checkClose(trade); ok {
				net, profit := position.Value()
				msg := fmt.Sprintf("%s %s:%s (%s)",
					emoji.MapToSign(net),
					string(trade.Coin),
					coinmath.Format(profit),
					coinmath.Format(net))
				user.Send(api.Private, api.NewMessage(msg), &api.Trigger{
					ID:      position.ID,
					Key:     positionKey,
					Default: []string{"?p", string(k.coin), "close", k.id},
					// TODO : instead of a big timeout check again when we want to close how the position is doing ...
				})
			}
			out <- trade
		}
		log.Info().Str("processor", positionProcessorName).Msg("closing processor")
	}
}

type closingConfig struct {
	coin model.Coin
	// profit is the profit percentage
	profit float64
	// stopLoss is the stop loss percentage
	stopLoss float64
	// Live defines the current / live status of the coin pair
	Live float64
	// highProfit is the max profit , under which we should sell
	highProfit float64
	// lowLoss is th low loss,
	lowLoss float64
	// toClose defines if the position is already flagged for closing
	toClose bool
	// blockClose defines if the position should be blocked for closing
	blockClose bool
}

var defaultClosingConfig = map[model.Coin]closingConfig{
	model.BTC: {
		coin:     model.BTC,
		profit:   1,
		stopLoss: 1.5,
	},
}

func getConfiguration(coin model.Coin) closingConfig {
	if cfg, ok := defaultClosingConfig[coin]; ok {
		return cfg
	}
	// create a new one from copying the btc
	cfg := defaultClosingConfig[model.BTC]
	return closingConfig{
		coin:       coin,
		profit:     cfg.profit,
		highProfit: cfg.profit,
		stopLoss:   cfg.stopLoss,
		lowLoss:    cfg.stopLoss,
	}
}
