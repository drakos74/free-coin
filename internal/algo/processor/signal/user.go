package signal

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

func (t *trader) compoundKey(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, t.account)
}

func (t *trader) switchOnOff(user api.User) {
	for command := range user.Listen(t.compoundKey(OnOffSwitch), "?r") {

		if t.account != "" && command.User != t.account {
			continue
		}

		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?r"),
			api.OneOf(&action, "start", "stop", ""),
		)

		if err != nil {
			log.Warn().
				Err(err).
				Str("text", command.Content).
				Str("user", command.User).
				Msg("could not process user message")
			api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "error")).ReplyTo(command.ID), err)
			continue
		}

		switch action {
		case "start":
			t.running = true
		case "stop":
			t.running = false
		}
		log.Info().
			Bool("running", t.running).
			Str("text", command.Content).
			Str("user", command.User).
			Msg("processed message")
		api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), emoji.MapBool(t.running))).ReplyTo(command.ID), nil)
	}
}

func (t *trader) trackUserActions(client api.Exchange, user api.User) {
	for command := range user.Listen(t.compoundKey(ProcessorName), "?p") {

		if t.account != "" && t.account != command.User {
			continue
		}

		ctx := context.Background()

		errMsg := ""
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?p"),
		)

		if err != nil {
			log.Warn().
				Err(err).
				Str("text", command.Content).
				Str("user", command.User).
				Msg("could not process user message")
			api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "error")).ReplyTo(command.ID), err)
			continue
		}

		keys, positions, prices := t.getAll(ctx)

		// get account balance first to double check ...
		bb, err := client.Balance(ctx, prices)
		if err != nil {
			errMsg = err.Error()
		}

		sort.Strings(keys)
		now := time.Now()

		posTotal := model.Balance{}
		report := api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "positions"))
		if len(positions) == 0 {
			report.AddLine("no open positions")
		}
		for i, k := range keys {
			pos := positions[k]
			since := now.Sub(pos.OpenTime)
			configMsg := fmt.Sprintf("[ %s ] [ %.0fh ]", k, math.Round(since.Hours()))
			msg := createPositionMessage(i, pos, bb[pos.Coin])

			if balance, ok := bb[pos.Coin]; ok {
				balance.Volume -= pos.Volume
				bb[pos.Coin] = balance
			}
			posTotal.Locked += pos.OpenPrice * pos.Volume
			posTotal.Volume += pos.CurrentPrice * pos.Volume
			// TODO : send a trigger for each Position to give access to adjust it
			//trigger := &api.Trigger{
			//	ID:  pos.ID,
			//	Key: positionKey,
			//}
			report = report.AddLine(msg).AddLine(configMsg).AddLine("************")
			if errMsg != "" {
				report = report.AddLine(fmt.Sprintf("balance:error:%s", errMsg))
			}
		}
		// send all positions report ... to avoid spamming the chat
		user.Send(api.Index(command.User), report, nil)

		balanceTotal := model.Balance{}
		balanceReport := api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "balance"))
		for coin, balance := range bb {
			// bad one ... but its difficult to recognise when we have no value
			if math.Abs(balance.Volume) > 0.000000001 {
				balanceReport = balanceReport.AddLine(fmt.Sprintf("%s %f -> %f%s",
					string(coin),
					balance.Volume,
					balance.Volume*balance.Price,
					emoji.Money))
			}
			if balance.Price < 0.0000001 {
				balanceTotal.Locked += balance.Volume
			} else {
				balanceTotal.Locked += balance.Volume * balance.Price
			}
			balanceTotal.Volume += balance.Volume
		}
		// print also the posTotal ...
		balanceReport.AddLine(createBalanceTotal(posTotal, balanceTotal))
		user.Send(api.Index(command.User), balanceReport, nil)
	}
}

func createPositionMessage(i int, pos model.Position, balance model.Balance) string {
	net, profit := pos.Value()
	return fmt.Sprintf("[%d] %s %.2f%s (%.2f%s) <- %s | %f [%f]",
		i+1,
		emoji.MapToSign(net),
		profit,
		"%",
		pos.OpenPrice,
		emoji.Money,
		emoji.MapType(pos.Type),
		pos.Volume,
		balance.Volume,
	)
}

func createBalanceTotal(total model.Balance, netTotal model.Balance) string {
	v := (total.Volume - total.Locked) / total.Locked
	w := (total.Volume - total.Locked) / (total.Locked + netTotal.Locked)
	return fmt.Sprintf("%s(%.2f%s|%.2f%s) %f%s -> %f%s | %f%s",
		emoji.MapValue(10*v/2),
		100*v, "%",
		100*w, "%",
		total.Locked, emoji.Money,
		total.Volume, emoji.Money,
		total.Locked+netTotal.Locked, emoji.Money)
}
