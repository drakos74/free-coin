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

func trackUserActions(index api.Index, user api.User, collector *collector, strategy *strategy, wallet *trader.ExchangeTrader, benchmarks *Benchmark, config *Config) {
	for command := range user.Listen("ml", "?ml") {
		log.Debug().
			Str("user", command.User).
			Str("message", command.Content).
			Str("processor", Name).
			Msg("message received")
		var num float64
		var coin string
		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?ml"),
			api.OneOf(&action, "start", "stop", "reset", "pos", "tp", "sl", "ov", "cfg", "wallet", "gap", "prec", ""),
			api.Any(&coin),
			api.Float(&num),
		)
		if err != nil {
			api.Reply(index, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}

		key := model.TempKey(model.Coin(strings.ToUpper(coin)), time.Duration(num)*time.Minute)

		txtBuffer := new(strings.Builder)

		switch action {
		case "":
			_, positions := wallet.CurrentPositions()
			for k, bench := range benchmarks.Profit {
				if key.Coin == model.NoCoin || key.Match(k.Coin) {
					for _, report := range bench {
						pString := ""
						if p, ok := positions[k]; ok {
							pString = fmt.Sprintf("%s\n", formatPosition(p))
						} else {
							pString = "<none>"
						}
						txtBuffer.WriteString(fmt.Sprintf("%s|%s %s(%s)\n%s\n%s\n",
							report.Stamp.Format(time.Stamp),
							k.ToString(),
							emoji.MapOpen(strategy.enabled[k.Coin]),
							emoji.MapToSign(report.Profit),
							pString,
							formatReport(report)))
					}
				}
			}
		case "wallet":
			settings := wallet.Settings()
			txtBuffer.WriteString(formatSettings(settings))
		case "cfg":
			txtBuffer.WriteString(formatConfig(*config))
		case "gap":
			c := config.SetGap(key.Coin, num)
			txtBuffer.WriteString(formatConfig(*c))
		case "prec":
			c := config.SetPrecisionThreshold(key.Coin, num)
			txtBuffer.WriteString(formatConfig(*c))
		case "tp":
			settings := wallet.TakeProfit(num / 100)
			txtBuffer.WriteString(formatSettings(settings))
		case "sl":
			settings := wallet.StopLoss(num / 100)
			txtBuffer.WriteString(formatSettings(settings))
		case "ov":
			settings := wallet.OpenValue(num)
			txtBuffer.WriteString(formatSettings(settings))
		case "stop":
			bb := strategy.enable(key.Coin, false)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		case "start":
			bb := strategy.enable(key.Coin, true)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		case "reset":
			pp, err := wallet.UpstreamPositions(context.Background())
			if err != nil {
				txtBuffer.WriteString(fmt.Sprintf("err=<%s>\n", err.Error()))
			} else {
				kk, positions := wallet.CurrentPositions(key.Coin)
				for _, k := range kk {
					position := positions[k]
					for _, p := range pp {
						if k.Match(p.Coin) {
							if position.Type == p.Type {
								// the least we can check here ...
								_, ok, _, err := wallet.CreateOrder(k, time.Now(), position.OpenPrice, p.Type.Inv(), false, p.Volume, trader.ForceResetReason, true)
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
			i, err := wallet.Reset(key.Coin)
			txtBuffer.WriteString(fmt.Sprintf("%d:%+v", i, err))
		case "pos":
			pp, err := wallet.UpstreamPositions(context.Background())
			if err != nil {
				txtBuffer.WriteString(fmt.Sprintf("err=<%s>\n", err.Error()))
			}
			kk, positions := wallet.CurrentPositions(key.Coin)
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
						ep := np.Update(model.Tick{
							Level: model.Level{
								Price: pos[0].p.CurrentPrice,
							},
							Time: pos[0].p.CurrentTime,
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
