package ml

import (
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/trader"

	"github.com/drakos74/free-coin/internal/emoji"

	"github.com/drakos74/free-coin/internal/model"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/rs/zerolog/log"
)

func trackUserActions(index api.Index, user api.User, collector *collector, strategy *strategy, trader *trader.ExchangeTrader) {
	for command := range user.Listen("ml", "?ml") {
		log.Debug().
			Str("user", command.User).
			Str("message", command.Content).
			Str("processor", Name).
			Msg("message received")
		var duration int
		var coin string
		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?ml"),
			api.Any(&coin),
			api.Int(&duration),
			api.OneOf(&action, "start", "stop", ""),
		)
		if err != nil {
			api.Reply(index, user, api.NewMessage("[cmd error]").ReplyTo(command.ID), err)
			continue
		}

		key := model.TempKey(model.Coin(strings.ToUpper(coin)), time.Duration(duration)*time.Minute)

		txtBuffer := new(strings.Builder)

		switch action {
		case "":
			_, positions := trader.Positions()
			for k, report := range strategy.report {
				if key.Coin == model.NoCoin || k.Match(key.Coin) {
					pString := ""
					if p, ok := positions[k]; ok {
						pString = fmt.Sprintf("%s\n", formatPosition(p))
					}
					txtBuffer.WriteString(fmt.Sprintf("%s:%.fm %s|%s\n(%s)%s\n",
						k.Coin,
						k.Duration.Minutes(),
						emoji.MapOpen(strategy.config[k].enabled),
						pString,
						emoji.MapToSign(report.Profit),
						formatReport(report)))
				}
			}
		case "stop":
			bb := strategy.enable(model.Coin(coin), false)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		case "start":
			bb := strategy.enable(model.Coin(coin), true)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		}

		api.Reply(index, user,
			api.NewMessage(fmt.Sprintf("(%s|%s) - %s",
				Name,
				command.User,
				txtBuffer.String()),
			), nil)
	}
}
