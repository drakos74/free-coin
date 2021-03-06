package signal

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	PositionsReport = "signal-positions"
	OnOffSwitch     = "signal-on-off"
	Configuration   = "signal-configuration"
	configPrefix    = "?m"
)

func (t *trader) compoundKey(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, t.account)
}

func (t *trader) configure(user api.User) {
	for command := range user.Listen(t.compoundKey(Configuration), configPrefix) {

		if t.account != "" && command.User != t.account {
			continue
		}

		var base int
		var pair string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains(configPrefix),
			api.Int(&base),
			api.Any(&pair),
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

		//coin := model.Coin(pair)
		//
		//// adjust the multiplier ...
		//match := func(c model.Coin) bool {
		//	if coin == "" {
		//		return true
		//	}
		//	return coin == c
		//}
		//err = t.updateConfig(multiplier, match)
		//if err != nil {
		//	api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "error")).ReplyTo(command.ID), err)
		//}

		if base > 0 {
			t.minSize = base
		}
		err = t.save()

		// build the report message
		var sb strings.Builder
		for c, cfg := range t.config {
			sb.WriteString(fmt.Sprintf("%s : %s %s", c, cfg.String(), "\n"))
		}
		sb.WriteString(fmt.Sprintf("min-size : %d", t.minSize))
		cfg := sb.String()
		msg := api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), cfg))
		if err != nil {
			msg.AddLine(fmt.Sprintf("error [%v]", err))
		}
		api.Reply(api.Index(command.User), user, msg.ReplyTo(command.ID), nil)
	}
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
		api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), emoji.MapBool(t.running))).ReplyTo(command.ID), nil)
	}
}

func (t *trader) portfolio(client api.Exchange, user api.User) {
	for command := range user.Listen(t.compoundKey(ProcessorName), "?p") {

		if t.account != "" && t.account != command.User {
			continue
		}

		ctx := context.Background()

		errMsg := ""
		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?p"),
			api.OneOf(&action, "reset", ""),
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

		if action == "reset" {
			positions, err = t.reset("")
			if err != nil {
				api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "reset error")).ReplyTo(command.ID), err)
			}
		}

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

func (t *trader) trade(client api.Exchange, user api.User) {
	for command := range user.Listen(t.compoundKey(ProcessorName), "?t") {

		if t.account != "" && t.account != command.User {
			continue
		}

		ctx := context.Background()

		var c string
		var budget string
		var action string
		_, err := command.Validate(
			api.AnyUser(),
			api.Contains("?t"),
			api.NotEmpty(&c),
			api.OneOf(&action /* "buy", "to", "sell", */, "from"),
			api.NotEmpty(&budget),
		)

		if err != nil {
			log.Warn().
				Err(err).
				Str("text", command.Content).
				Str("user", command.User).
				Msg("could not process user message")
			api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "command error")).ReplyTo(command.ID), err)
			continue
		}

		// get balance to check availability
		bb, err := client.Balance(ctx, nil)
		if err != nil {
			api.Reply(api.Index(command.User), user, api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "balance error")).ReplyTo(command.ID), err)
			continue
		}

		pairs := client.Pairs(context.Background())

		// execute the command
		completed := make([]model.Coin, 0)
		report := api.NewMessage(processor.Audit(t.compoundKey(ProcessorName), "trader"))
		for _, balance := range bb {
			pair, err := matchesBalance(budget, c, balance.Coin)
			// dont make manual trades with BNB and USDT
			if strings.Contains(pair, "BNB") || strings.HasPrefix(pair, "USDT") {
				report.AddLine(fmt.Sprintf("ignore:%s:%s", pair, "invalid"))
				report.AddLine("**********")
				continue
			}
			coin := model.Coin(pair)
			if err == nil {
				if _, ok := pairs[pair]; !ok {
					report.AddLine(fmt.Sprintf("error:%s:%s", pair, "unknown"))
					report.AddLine("**********")
					continue
				}
				//build the pair ...
				order := model.NewOrder(coin).
					WithType(model.TypeFromString(action)).
					Market().
					WithVolume(balance.Volume).
					CreateTracked(model.Key{
						Coin:     model.Coin(c),
						Index:    sellOff,
						Strategy: fmt.Sprintf("command:%s:%s", command.Content, command.User),
					}, time.Now())
				o, _, err := client.OpenOrder(order)
				if err != nil {
					report.AddLine(fmt.Sprintf("error:%s:%s", pair, err.Error()))
				} else {
					completed = append(completed, coin)
					report.AddLine(fmt.Sprintf("%s %.4f %s for %.4f", emoji.MapType(o.Type), o.Volume, o.Coin, o.Price))
				}
				report.AddLine("**********")
			} else {
				log.Warn().
					Err(err).
					Str("pair", pair).
					Str("coin", c).
					Str("budget", budget).
					Str("coin", string(balance.Coin)).
					Msg("no match")
			}
		}
		// clear the related positions
		newPositions, err := t.reset(completed...)
		if err != nil {
			report.AddLine(fmt.Sprintf("reset error = %s", err.Error()))
		} else {
			report.AddLine(fmt.Sprintf("new positions = %d", len(newPositions)))
		}
		report.AddLine("**********")
		user.Send(api.Index(command.User), report, nil)
	}
}

func matchesBalance(budget, coin string, balance model.Coin) (string, error) {
	if !strings.HasSuffix(string(balance), strings.ToUpper(coin)) {
		return "", fmt.Errorf("no valid coin '%s' in '%s'", coin, balance)
	}
	if budget == "all" {
		return string(balance), nil
	}
	pair := strings.ToUpper(fmt.Sprintf("%s%s", budget, coin))
	if string(balance) != pair {
		return "", fmt.Errorf("could not match pair: %s vs %s", pair, balance)
	}
	return string(balance), nil
}

func createPositionMessage(i int, pos model.Position, balance model.Balance) string {
	net, profit := pos.Value(nil)
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
