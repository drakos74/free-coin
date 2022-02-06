package ml

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/trader"
	"github.com/rs/zerolog/log"
)

func trackUserActions(index api.Index, user api.User, collector *collector, strategy *strategy, trader *trader.ExchangeTrader) {
	for command := range user.Listen("ml", "?ml") {
		log.Debug().
			Str("user", command.User).
			Str("message", command.Content).
			Str("processor", Name).
			Msg("message received")
		var num int
		var coin string
		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?ml"),
			api.OneOf(&action, "start", "stop", "reset", "pos", "tp", "sl", "ov", "cfg", ""),
			api.Any(&coin),
			api.Int(&num),
		)
		if err != nil {
			api.Reply(index, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}

		key := model.TempKey(model.Coin(strings.ToUpper(coin)), time.Duration(num)*time.Minute)

		txtBuffer := new(strings.Builder)

		switch action {
		case "":
			_, positions := trader.CurrentPositions()
			txtBuffer.WriteString(fmt.Sprintf("%d:%d\n", len(positions), len(strategy.report)))
			for k, report := range strategy.report {
				if key.Coin == model.NoCoin || k.Match(key.Coin) {
					pString := ""
					if p, ok := positions[k]; ok {
						pString = fmt.Sprintf("%s\n", formatPosition(p))
					} else {
						pString = "<none>"
					}
					txtBuffer.WriteString(fmt.Sprintf("%s:%.fm %s(%s)\n%s\n%s\n",
						k.Coin,
						k.Duration.Minutes(),
						emoji.MapOpen(strategy.enabled[k.Coin]),
						emoji.MapToSign(report.Profit),
						pString,
						formatReport(report)))
				}
			}
		case "cfg":
			settings := trader.Settings()
			txtBuffer.WriteString(formatSettings(settings))
		case "tp":
			settings := trader.TakeProfit(float64(num) / 100)
			txtBuffer.WriteString(formatSettings(settings))
		case "sl":
			settings := trader.StopLoss(float64(num) / 100)
			txtBuffer.WriteString(formatSettings(settings))
		case "ov":
			settings := trader.OpenValue(float64(num))
			txtBuffer.WriteString(formatSettings(settings))
		case "stop":
			bb := strategy.enable(key.Coin, false)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		case "start":
			bb := strategy.enable(key.Coin, true)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		case "reset":
			pp, err := trader.UpstreamPositions(context.Background())
			if err != nil {
				txtBuffer.WriteString(fmt.Sprintf("err=<%s>\n", err.Error()))
			} else {
				kk, positions := trader.CurrentPositions(key.Coin)
				for _, k := range kk {
					position := positions[k]
					for _, p := range pp {
						if k.Match(p.Coin) {
							if position.Type == p.Type {
								// the least we can check here ...
								_, ok, _, err := trader.CreateOrder(k, time.Now(), position.OpenPrice, p.Type.Inv(), false, p.Volume)
								if err != nil || !ok {
									txtBuffer.WriteString(fmt.Sprintf("%v|err=<%s>\n", ok, err.Error()))
								} else {
									break
								}
							}
						}
					}
				}
			}
			i, err := trader.Reset(key.Coin)
			txtBuffer.WriteString(fmt.Sprintf("%d:%+v", i, err))
		case "pos":
			pp, err := trader.UpstreamPositions(context.Background())
			if err != nil {
				txtBuffer.WriteString(fmt.Sprintf("err=<%s>\n", err.Error()))
			}
			kk, positions := trader.CurrentPositions(key.Coin)
			txtBuffer.WriteString(fmt.Sprintf("%d:%d\n", len(pp), len(kk)))
			coinPositions := make(map[model.Coin][]position)
			for _, k := range kk {
				if _, ok := coinPositions[k.Coin]; !ok {
					coinPositions[k.Coin] = make([]position, 0)
				}
				coinPositions[k.Coin] = append(coinPositions[k.Coin], position{
					p: positions[k],
					k: k,
				})
			}
			for c, pos := range coinPositions {
				internalSum := 0.0
				internalValue := 0.0
				externalSum := 0.0
				externalValue := 0.0
				externalCount := 0
				for _, np := range pp {
					if np.Coin == c {
						ep := np.Update(&model.Trade{
							Price: pos[0].p.CurrentPrice,
							Time:  pos[0].p.CurrentTime,
						})
						externalValue += ep.Cost
						externalSum += ep.PnL
						externalCount++
					}
				}
				for _, ip := range pos {
					internalSum += ip.p.PnL
					internalValue += ip.p.OpenPrice * ip.p.Volume
					txtBuffer.WriteString(fmt.Sprintf("%s:%.fm %s\n",
						ip.k.Coin,
						ip.k.Duration.Minutes(),
						formatPosition(ip.p)))
				}
				txtBuffer.WriteString(fmt.Sprintf("value = %.2f | pnl = %.2f (%d:%d)\n",
					externalValue-internalValue,
					externalSum-internalSum,
					externalCount,
					len(pos),
				))
			}
		}

		api.Reply(index, user,
			api.NewMessage(fmt.Sprintf("(%s|%s) - %s",
				Name,
				command.User,
				txtBuffer.String()),
			), nil)
	}
}

type position struct {
	p model.Position
	k model.Key
}
