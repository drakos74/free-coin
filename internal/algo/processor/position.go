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

type tradeAction struct {
	key      tpKey
	position *tradePosition
	doClose  bool
}

type tradePositions struct {
	pos  map[model.Coin]map[string]*tradePosition
	lock *sync.RWMutex
}

func (tp *tradePositions) checkClose(trade *model.Trade) []tradeAction {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	actions := make([]tradeAction, 0)
	if positions, ok := tp.pos[trade.Coin]; ok {
		for id, p := range positions {
			p.position.CurrentPrice = trade.Price
			if trade.Live {
				net, profit := p.position.Value()
				log.Debug().
					Str("coin", string(p.position.Coin)).
					Float64("net", net).
					Float64("profit", profit).
					Msg("check position")
				actions = append(actions, tradeAction{
					key: tpKey{
						coin: trade.Coin,
						id:   id,
					},
					position: p,
					doClose:  p.DoClose(),
				})
			}
		}
	}
	for _, action := range actions {
		tp.pos[action.key.coin][action.key.id] = action.position
	}
	return actions
}

func (tp *tradePositions) updateConfig(key tpKey, profit, stopLoss float64) {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	tp.pos[key.coin][key.id].config.profit = profit
	tp.pos[key.coin][key.id].config.stopLoss = stopLoss
}

func (tp *tradePositions) update(client api.Exchange) error {
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

func (tp *tradePositions) track(client api.Exchange, ticker *time.Ticker, quit chan struct{}, block api.Block) {
	// and update the positions at the predefined interval.
	for {
		select {
		case <-block.Action: // trigger update on the positions on external events/actions
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
			}
			block.ReAction <- api.Action{}
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
			tp.close(client, user, k)
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
			for coin, pos := range tp.getAll() {
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

func (tp *tradePositions) getAll() map[model.Coin]map[string]tradePosition {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	positions := make(map[model.Coin]map[string]tradePosition)
	for c, pp := range tp.pos {
		positions[c] = make(map[string]tradePosition)
		for id, p := range pp {
			positions[c][id] = *p
		}
	}
	return positions
}

func (tp *tradePositions) delete(k tpKey) {
	ttp := make(map[model.Coin]map[string]*tradePosition)
	for c, positions := range tp.pos {
		if _, ok := ttp[c]; !ok {
			ttp[c] = make(map[string]*tradePosition)
		}
		for id, pos := range positions {
			key := tpKey{coin: c, id: id}
			if key != k {
				ttp[c][id] = pos
			}
		}
	}
	tp.pos = ttp
	//delete(tp.pos[k.coin], k.id)
	fmt.Println(fmt.Sprintf("delete = %+v", k))
}

func (tp *tradePositions) exists(key tpKey) bool {
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
func Position(client api.Exchange, user api.User, block api.Block, closeInstant bool) api.Processor {

	// define our internal global statsCollector
	positions := tradePositions{
		pos:  make(map[model.Coin]map[string]*tradePosition),
		lock: new(sync.RWMutex),
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
			for _, positionAction := range positions.checkClose(trade) {
				if positionAction.doClose {
					if closeInstant {
						positions.close(client, user, positionAction.key)
					} else {
						net, profit := positionAction.position.position.Value()
						msg := fmt.Sprintf("%s %s:%s (%s)",
							emoji.MapToSign(net),
							string(trade.Coin),
							coinmath.Format(profit),
							coinmath.Format(net))
						user.Send(api.Private, api.NewMessage(msg), &api.Trigger{
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
		log.Info().Str("processor", positionProcessorName).Msg("closing processor")
	}
}

func (tp *tradePositions) close(client api.Exchange, user api.User, key tpKey) bool {
	tp.lock.Lock()
	defer tp.lock.Unlock()
	// ok, here we might encounter the case that we closed the position from another trigger
	if _, ok := tp.pos[key.coin]; !ok {
		return false
	}
	if _, ok := tp.pos[key.coin][key.id]; !ok {
		return false
	}
	position := tp.pos[key.coin][key.id].position
	net, profit := position.Value()
	err := client.ClosePosition(position)
	if err != nil {
		log.Error().
			Float64("volume", position.Volume).
			Str("id", key.id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("could not close position")
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("could not close %s [%s]: %s", key.coin, key.id, err.Error())), nil)
		return false
	} else {
		log.Info().
			Float64("volume", position.Volume).
			Str("id", key.id).
			Str("coin", string(position.Coin)).
			Float64("net", net).
			Float64("profit", profit).
			Msg("closed position")
		tp.delete(key)
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("closed %s at %.2f [%s]", key.coin, profit, key.id)), nil)
		return true
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
		profit:   0.8,
		stopLoss: 2.0,
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
