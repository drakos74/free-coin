package ml

import (
	"context"
	"fmt"
	"strings"
	"time"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/trader"
	"github.com/rs/zerolog/log"
)

func trackUserActions(index api.Index, user api.User, collector *collector, strategy *strategy, wallet *trader.ExchangeTrader, config *mlmodel.Config) {
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
			api.OneOf(&action,
				"start",
				"stop",
				"close",
				"pos",
				"tp",
				"sl",
				"ov",
				"cfg",
				"wallet",
				"gap",
				"prec",
				"ds",
				"stats",
				"log",
				"",
			),
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
		case "wallet":
			settings := wallet.Settings()
			txtBuffer.WriteString(formatSettings(settings))
		case "stats":
			coinStats, networkStats := wallet.Stats()
			if key.Coin == model.AllCoins || key.Coin == model.NoCoin {
				for c, stat := range coinStats {
					txtBuffer.WriteString(formatStat(string(c), stat))
				}
			} else {
				txtBuffer.WriteString(formatStat(string(key.Coin), coinStats[key.Coin]))
			}
			for network, stat := range networkStats {
				txtBuffer.WriteString(formatStat(network, stat))
			}
		case "cfg":
			txtBuffer.WriteString(formatConfig(*config))
		case "gap":
			c := config.SetGap(key.Coin, num)
			txtBuffer.WriteString(formatConfig(*c))
		case "prec":
			c := config.SetPrecisionThreshold(key.Coin, num)
			txtBuffer.WriteString(formatConfig(*c))
		case "log":
			settings := wallet.TakeProfit(num / 100)
			txtBuffer.WriteString(formatSettings(settings))
		case "tp":
			settings := wallet.TakeProfit(num / 100)
			txtBuffer.WriteString(formatSettings(settings))
		case "sl":
			settings := wallet.StopLoss(num / 100)
			txtBuffer.WriteString(formatSettings(settings))
		case "ov":
			settings := wallet.OpenValue(num)
			txtBuffer.WriteString(formatSettings(settings))
		case "ds":
			sets := strategy.datasets.Sets()
			txtBuffer.WriteString(fmt.Sprintf("%d\n", len(sets)))
			for k, set := range sets {
				if model.IsAnyCoin(key.Coin) || k.Match(key.Coin) {
					networks := set.Network
					txtBuffer.WriteString(fmt.Sprintf("%+v\n", k.ToString()))
					for kk, network := range networks.Networks {
						txtBuffer.WriteString(fmt.Sprintf("%+v\n", network.Model().Format()))

						trend := networks.Trend[kk]
						report := network.Report()
						txtBuffer.WriteString(fmt.Sprintf("%.2f%s | %d (%.2f)\n",
							report.Profit, emoji.Profit, report.Buy+report.Sell,
							trend))

						stats := network.Stats()
						aa := make([]string, len(stats.Accuracy))
						for i, acc := range stats.Accuracy {
							aa[i] = fmt.Sprintf("%.2f|%s", acc, emoji.MapType(model.Type(stats.Decisions[i])))
						}
						txtBuffer.WriteString(fmt.Sprintf("(%d) %+v\n", stats.Iterations, aa))
					}
					//txtBuffer.WriteString(fmt.Sprintf("%+v\n", set.Network.Stats))
					//txtBuffer.WriteString(fmt.Sprintf("%+v\n", set.Network.Performance))
				}
			}
		case "start":
			bb := strategy.enable(key.Coin, true)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		case "stop":
			bb := strategy.enable(key.Coin, false)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		case "close":
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
								_, ok, _, err := wallet.CreateOrder(k, time.Now(), position.OpenPrice, p.Type.Inv(), false, p.Volume, trader.ForceResetReason, true, nil)
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
				internalPnl := 0.0
				internalValue := 0.0
				externalPnl := 0.0
				externalValue := 0.0
				externalCount := 0
				internalOpen := 0.0
				externalOpen := 0.0
				internalVolume := 0.0
				externalVolume := 0.0
				for _, np := range pp {
					if np.Coin == c {
						externalVolume += np.Volume
						externalOpen += np.OpenPrice
						externalValue += np.Net
						externalCount++
					}
				}
				for _, ip := range pos {
					internalVolume += ip.p.Volume
					internalOpen += ip.p.OpenPrice
					internalValue += ip.p.OpenPrice * ip.p.Volume
					internalPnl += ip.p.PnL
					pnl, _, _ := model.PnL(ip.p.Type, externalVolume, externalOpen, ip.p.CurrentPrice)
					externalPnl += pnl
					txtBuffer.WriteString(fmt.Sprintf("%s %s\n",
						ip.k.Coin,
						//ip.k.Duration.Minutes(),
						formatPosition(ip.p)))
				}
				//txtBuffer.WriteString(fmt.Sprintf("value = %.2f | pnl = %.2f (%d:%d)\n",
				//	externalValue-internalValue,
				//	externalPnl-internalPnl,
				//	externalCount,
				//	len(pos),
				//))
				//txtBuffer.WriteString(fmt.Sprintf("ext-open = %.2f | int-open = %.2f | (%.2f)\n"+
				//	"ext-vol = %f | int-vol = %f\n"+
				//	"ext-pnl = %.2f | int-pnl = %.2f\n"+
				//	"ext-value = %.2f | int-value = %.2f\n",
				//	externalOpen,
				//	internalOpen,
				//	internalOpen-externalOpen,
				//	externalVolume,
				//	internalVolume,
				//	externalPnl,
				//	internalPnl,
				//	externalValue,
				//	internalValue,
				//))
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
