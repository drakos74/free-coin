package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/math"
	"github.com/rs/zerolog/log"
)

const (
	positionRefreshInterval = 5 * time.Minute
	positionProcessorName   = "position"
)

// PositionTracker is the processor responsible for tracking open positions and acting on previous triggers.
func Position(client model.TradeClient, user model.UserInterface) api.Processor {

	// define our internal global state
	var positions tradePositions = make(map[api.Coin]map[string]*tradePosition)

	ticker := time.NewTicker(positionRefreshInterval)
	quit := make(chan struct{})
	go positions.track(client, ticker, quit)

	go positions.trackUserActions(client, user)

	err := positions.update(client)
	if err != nil {
		log.Error().Err(err).Msg("could not get initial positions")
	}

	return func(in <-chan *api.Trade, out chan<- *api.Trade) {

		defer func() {
			log.Info().Str("processor", positionProcessorName).Msg("closing' strategy")
			close(out)
		}()

		for trade := range in {
			// check on the existing positions
			// ignore if it s not the closing trade of the batch
			if !trade.Active {
				continue
			}

			// TODO : integrate the above results to the 'Live' parameter
			for id := range positions[trade.Coin] {
				// update the position current price
				k := key(trade.Coin, id)
				positions.updatePrice(k, trade.Price)
				positions.checkClosePosition(client, user, k)
			}
		}
		log.Info().Str("processor", positionProcessorName).Msg("closing processor")
	}
}

type tpKey struct {
	coin api.Coin
	id   string
}

func key(c api.Coin, id string) tpKey {
	return tpKey{
		coin: c,
		id:   id,
	}
}

type tradePositions map[api.Coin]map[string]*tradePosition

func (tp tradePositions) updatePrice(key tpKey, price float64) {
	tp[key.coin][key.id].position.CurrentPrice = price
}

func (tp tradePositions) updateConfig(key tpKey, profit, stopLoss float64) {
	tp[key.coin][key.id].config.profit = profit
	tp[key.coin][key.id].config.stopLoss = stopLoss
}

func (tp tradePositions) update(client model.TradeClient) error {
	pp, err := client.OpenPositions(context.Background())
	if err != nil {
		return fmt.Errorf("could not get positions: %w", err)
	}
	for _, p := range pp.Positions {
		cfg := getConfiguration(p.Coin)
		if p, ok := tp[p.Coin][p.ID]; ok {
			cfg = p.config
		}
		tp[p.Coin][p.ID] = &tradePosition{
			position: p,
			config:   cfg,
		}
	}
	return nil
}

func (tp tradePositions) track(client model.TradeClient, ticker *time.Ticker, quit chan struct{}) {
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

func (tp tradePositions) trackUserActions(client model.TradeClient, user model.UserInterface) {
	for command := range user.Listen("trade", "?p") {
		var action string
		var coin string
		var defVolume float64
		err := command.Validate(
			api.AnyUser(),
			api.Contains("?p", "?pos", "?positions"),
			api.OneOf(&action, "buy", "sell", ""),
			api.NotEmpty(&coin),
			api.Float(&defVolume),
		)
		if err != nil {
			model.Reply(user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}

		c := api.Coin(coin)

		switch action {
		case "":
			// TODO : return the current open positions
			err := tp.update(client)
			if err != nil {
				log.Error().Err(err).Msg("could not get positions")
				model.Reply(user, api.NewMessage("[api error]").ReplyTo(command.ID), err)
			}
			i := 0
			for id, p := range tp[c] {
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
					math.Format(p.position.OpenPrice))
				// TODO : send a trigger for each position to give access to adjust it
				user.Send(api.NewMessage(msg).AddLine(configMsg), api.NewTrigger(tp.closePositionTrigger(client, key(p.position.Coin, id))))
				i++
			}
		case "config":
			// TODO : configure the defaults
		case "buy":
			// TODO : buy
		case "sell":
			// TODO : sell
		}
	}
}

func (tp tradePositions) checkClosePosition(client model.TradeClient, user model.UserInterface, key tpKey) {
	p := tp[key.coin][key.id]
	net, profit := p.position.Value()
	log.Debug().
		Str("ID", key.id).
		Str("coin", string(p.position.Coin)).
		Float64("net", net).
		Float64("profit", profit).
		Msg("check position")
	if p.config.DoClose(p.position) {
		msg := fmt.Sprintf("%s %s:%s (%s) -> %v",
			emoji.MapToSign(net),
			string(p.position.Coin),
			math.Format(profit),
			math.Format(net),
			math.Format(p.config.Live))

		user.Send(api.NewMessage(msg), &api.Trigger{
			ID:      p.position.ID,
			Default: []string{"close"},
			Exec:    tp.closePositionTrigger(client, key),
			Timeout: 1 * time.Minute,
		})
	} else {
		// TODO : check also for trailing stop-loss
	}
}

func (tp tradePositions) closePositionTrigger(client model.TradeClient, key tpKey) api.TriggerFunc {
	return func(command api.Command, options ...string) (string, error) {
		var action string
		var nProfit float64
		var nStopLoss float64
		err := command.Validate(
			api.AnyUser(),
			api.Contains("skip", "extend", "reverse", "close"),
			api.OneOf(&action, "buy", "sell", ""),
			api.Float(&nProfit),
			api.Float(&nStopLoss),
		)
		if err != nil {
			return "", fmt.Errorf("invalid command: %w", err)
		}
		position := tp[key.coin][key.id].position
		net, profit := position.Value()
		switch action {
		case "skip":
			// avoid to complete the trigger in any case
			return "no action", nil
		case "extend":
			// TODO : adjust profit and stop-loss
			tp.updateConfig(key, nProfit, nStopLoss)
			return fmt.Sprintf("adjusted [ profit = %f , stop-loss = %f]", nProfit, nStopLoss), nil
		case "reverse":
			position.Volume = 2 * position.Volume
			fallthrough
		case "close":
			// TODO : check also market conditions from enriched trades !!!
			err := client.ClosePosition(position)
			if err != nil {
				log.Error().Str("coin", string(position.Coin)).Float64("net", net).Float64("profit", profit).Msg("could not close position")
				return "", fmt.Errorf("could not complete command: %w", err)
			} else {
				log.Info().Str("coin", string(position.Coin)).Float64("net", net).Float64("profit", profit).Msg("closed position")
				delete(tp[key.coin], key.id)
				return fmt.Sprintf("closed position for %s at %v", string(position.Coin), math.Format(net)), nil
			}
		}
		return "", fmt.Errorf("only allowed commands are [ close , extend , reverse , skip ]")
	}
}

type tradePosition struct {
	position api.Position
	config   closingConfig
}

type closingConfig struct {
	coin api.Coin
	// profit is the profit percentage
	profit float64
	// stopLoss is the stop loss percentage
	stopLoss float64
	// Live defines the current / live status of the coin pair
	Live float64
}

// DoClose checks if the given position should be closed, based on the current configuration.
// TODO : test this logic
func (c *closingConfig) DoClose(position api.Position) bool {
	net, p := position.Value()
	if net > 0 && p > c.profit {
		// only close if the market is going down
		return c.Live <= 0
	}
	if net < 0 && p < c.stopLoss {
		// only close if the market is going up
		return c.Live >= 0
	}
	return false
}

var defaultClosingConfig = map[api.Coin]closingConfig{
	api.BTC: {
		coin:     api.BTC,
		profit:   1.5,
		stopLoss: -1.5,
	},
}

func getConfiguration(coin api.Coin) closingConfig {
	if cfg, ok := defaultClosingConfig[coin]; ok {
		return cfg
	}
	// create a new one from copying the btc
	cfg := defaultClosingConfig[api.BTC]
	return closingConfig{
		coin:     coin,
		profit:   cfg.profit,
		stopLoss: cfg.stopLoss,
	}
}
