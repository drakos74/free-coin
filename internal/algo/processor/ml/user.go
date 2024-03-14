package ml

import (
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

func trackUserActions(index api.Index, user api.User, strategy *processor.Strategy, tracker map[model.Key]*Tracker) {
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
				"cfg",
				"gap",
				"prec",
				"ds",
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
		case "ds":
			txtBuffer.WriteString(fmt.Sprintf("%d\n", len(tracker)))
			if track, ok := tracker[key]; ok {
				txtBuffer.WriteString(fmt.Sprintf("%s\n%s",
					formatOutPredictions(time.Now(), key, 0, track.Prediction, track.Performance),
					formatRecentData(track.Buffer.Get())))
			} else if key.Coin == model.AllCoins {
				for k, _ := range strategy.Config().Segments {
					if track, ok := tracker[k]; ok {
						txtBuffer.WriteString(fmt.Sprintf("%s\n%s\n",
							formatOutPredictions(time.Now(), key, 0, track.Prediction, track.Performance),
							formatRecentData(track.Buffer.Get())))
					} else {
						txtBuffer.WriteString(fmt.Sprintf("no ds yet for ... %+v\n", k))
					}
				}
			} else {
				txtBuffer.WriteString(fmt.Sprintf("no ds yet for ... %+v\n", key))
			}
		case "cfg":
			txtBuffer.WriteString(formatConfig(strategy.Config()))
		case "gap":
			c := strategy.SetGap(key.Coin, num)
			txtBuffer.WriteString(formatConfig(c))
		case "prec":
			c := strategy.SetPrecisionThreshold(key.Coin, "all", num)
			txtBuffer.WriteString(formatConfig(c))
		case "start":
			bb := strategy.EnableML(key.Coin, true)
			txtBuffer.WriteString(fmt.Sprintf("%+v", bb))
		case "stop":
			bb := strategy.EnableML(key.Coin, false)
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

type position struct {
	p model.Position
	k model.Key
}
