package processor

import (
	"context"
	"fmt"
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

// PositionTracker is the processor responsible for tracking open positions and acting on previous triggers.
func Position(client api.Exchange, user api.User) api.Processor {

	// define our internal global statsCollector
	var positions tradePositions = make(map[model.Coin]map[string]*tradePosition)

	ticker := time.NewTicker(positionRefreshInterval)
	quit := make(chan struct{})
	go positions.track(client, ticker, quit)

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
			for id := range positions[trade.Coin] {
				// update the position current price
				k := key(trade.Coin, id)
				positions.updatePrice(k, trade.Price)
				if trade.Live {
					positions.checkClosePosition(client, user, k)
				}
			}

			out <- trade
		}
		log.Info().Str("processor", positionProcessorName).Msg("closing processor")
	}
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

type tradePositions map[model.Coin]map[string]*tradePosition

func (tp tradePositions) updatePrice(key tpKey, price float64) {
	tp[key.coin][key.id].position.CurrentPrice = price
}

func (tp tradePositions) updateConfig(key tpKey, profit, stopLoss float64) {
	tp[key.coin][key.id].config.profit = profit
	tp[key.coin][key.id].config.stopLoss = stopLoss
}

func (tp tradePositions) update(client api.Exchange) error {
	pp, err := client.OpenPositions(context.Background())
	if err != nil {
		return fmt.Errorf("could not get positions: %w", err)
	}
	for _, p := range pp.Positions {
		cfg := getConfiguration(p.Coin)
		if p, ok := tp[p.Coin][p.ID]; ok {
			cfg = p.config
		}
		if _, ok := tp[p.Coin]; !ok {
			tp[p.Coin] = make(map[string]*tradePosition)
		}
		tp[p.Coin][p.ID] = &tradePosition{
			position: p,
			config:   cfg,
		}
	}
	return nil
}

func (tp tradePositions) track(client api.Exchange, ticker *time.Ticker, quit chan struct{}) {
	// and update the positions at the predefined interval.
	for {
		select {
		case <-ticker.C:
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
			}
		case <-quit:
			ticker.Stop()
			return
		}
	}
}

const noPositionMsg = "no open positions"

func (tp tradePositions) trackUserActions(client api.Exchange, user api.User) {
	for command := range user.Listen("position", "?p") {
		var action string
		var coin string
		var defVolume float64
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?p", "?pos", "?positions"),
			api.Any(&coin),
			api.OneOf(&action, "buy", "sell", ""),
			api.Float(&defVolume),
		)
		c := model.Coin(coin)

		log.Info().
			Str("action", action).
			Str("coin-argument", coin).
			Str("coin", string(c)).
			Float64("defVolume", defVolume).
			Err(err).
			Msg("received user action")
		if err != nil {
			api.Reply(api.Private, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}

		switch action {
		case "":
			// TODO : return the current open positions
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
				api.Reply(api.Private, user, api.NewMessage("[api error]").ReplyTo(command.ID), err)
			}
			i := 0
			if len(tp) == 0 {
				user.Send(api.Private, api.NewMessage(noPositionMsg), nil)
				continue
			}
			for coin, pos := range tp {
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
						trigger := api.NewTrigger(tp.closePositionTrigger(client, key(p.position.Coin, id)))
						user.Send(api.Private, api.NewMessage(msg).AddLine(configMsg), trigger)
						i++
					}
				}
			}
		}
	}
}

func (tp tradePositions) checkClosePosition(client api.Exchange, user api.User, key tpKey) {
	p := tp[key.coin][key.id]
	net, profit := p.position.Value()
	log.Info().
		Str("ID", key.id).
		Str("coin", string(p.position.Coin)).
		Float64("net", net).
		Float64("profit", profit).
		Msg("check position")
	if p.DoClose() {
		msg := fmt.Sprintf("%s %s:%s (%s) -> %v",
			emoji.MapToSign(net),
			string(p.position.Coin),
			coinmath.Format(profit),
			coinmath.Format(net),
			coinmath.Format(p.config.Live))
		user.Send(api.Private, api.NewMessage(msg), &api.Trigger{
			ID:      p.position.ID,
			Default: []string{"close"},
			Exec:    tp.closePositionTrigger(client, key),
			// TODO : instead of a big timeout check again when we want to close how the position is doing ...
		})
	} else {
		// TODO : check also for trailing stop-loss
	}
}

func (tp tradePositions) exists(key tpKey) bool {
	// ok, here we might encounter the case that we closed the position from another trigger
	if _, ok := tp[key.coin]; !ok {
		return false
	}
	if _, ok := tp[key.coin][key.id]; !ok {
		return false
	}
	return true
}

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
		if !tp[key.coin][key.id].DoClose() {
			log.Error().Str("id", key.id).Str("coin", string(key.coin)).Msg("position conditions reversed")
			return "", fmt.Errorf("position conditions reversed")
		}
		position := tp[key.coin][key.id].position
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
			return fmt.Sprintf("%s position for %s [ profit = %f , stop-loss = %f]", exec, string(position.Coin), tp[key.coin][key.id].config.profit, tp[key.coin][key.id].config.stopLoss), nil
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
				delete(tp[key.coin], key.id)
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
	println(fmt.Sprintf("net = %+v", net))
	println(fmt.Sprintf("p = %+v", p))
	if net > 0 && p > tp.config.profit {
		// check the previous profit in order to extend profit
		if p > tp.config.lastProfit {
			println(fmt.Sprintf("tp.config.lastProfit = %+v", tp.config.lastProfit))
			// if we are making more ... ignore
			tp.config.lastProfit = p
			return false
		}
		diff := tp.config.lastProfit - p
		if diff < 0.4 {
			// leave for now, hoping profit will go up again
			// but dont update our highest value
			return false
		}
		// only close if the market is going down
		return tp.config.Live <= 0
	}
	if net < 0 && p < -1*tp.config.stopLoss {
		if tp.config.lastLoss <= p {
			tp.config.lastLoss = p
			// we are improving our position ... so give it a bit of time.
			return false
		}
		tp.config.lastLoss = p
		// only close if the market is going up
		return tp.config.Live >= 0
	}
	// check if we missed a profit opportunity here
	return tp.config.lastProfit > 0
}

type closingConfig struct {
	coin model.Coin
	// profit is the profit percentage
	profit float64
	// stopLoss is the stop loss percentage
	stopLoss float64
	// Live defines the current / live status of the coin pair
	Live float64
	// lastProfit is the last net value, in order to track trailing stop-loss
	lastProfit float64
	// lastLoss is the last net loss value, in order to track reversing conditions
	lastLoss float64
	// toClose defines if the position is already flagged for closing
	toClose bool
	// blockClose defines if the position should be blocked for closing
	blockClose bool
}

var defaultClosingConfig = map[model.Coin]closingConfig{
	model.BTC: {
		coin:     model.BTC,
		profit:   1,
		stopLoss: 2.5,
	},
}

func getConfiguration(coin model.Coin) closingConfig {
	if cfg, ok := defaultClosingConfig[coin]; ok {
		return cfg
	}
	// create a new one from copying the btc
	cfg := defaultClosingConfig[model.BTC]
	return closingConfig{
		coin:     coin,
		profit:   cfg.profit,
		stopLoss: cfg.stopLoss,
	}
}
