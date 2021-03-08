package coin

import (
	"fmt"
	"strings"

	"github.com/drakos74/free-coin/internal/buffer"

	"github.com/drakos74/free-coin/cmd/backtest/model"
	"github.com/drakos74/free-coin/internal/algo/processor/trade"
	"github.com/drakos74/free-coin/internal/emoji"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
)

func TrackingOrder(order coinmodel.TrackingOrder) model.AnnotationInstance {
	return model.AnnotationInstance{
		Title: fmt.Sprintf("%2.f %.2f", order.Price, order.Volume),
		Text:  fmt.Sprintf("%s %v", order.ID, order.TxIDs),
		Time:  cointime.ToMilli(order.Time),
		Tags:  []string{order.Type.String()},
	}
}

func TrackedPosition(position coinmodel.TrackedPosition) model.AnnotationInstance {
	net, profit := position.Value()
	return model.AnnotationInstance{
		Title:    fmt.Sprintf("%2.f (%.2f)", net, profit),
		Text:     fmt.Sprintf("%s %v", emoji.MapToSign(profit), position.Position.ID),
		Time:     cointime.ToMilli(position.Open),
		TimeEnd:  cointime.ToMilli(position.Close),
		IsRegion: true,
		Tags:     []string{position.Position.Type.String(), emoji.MapToSign(profit)},
	}
}

func PredictionPair(pair trade.PredictionPair) model.AnnotationInstance {
	return model.AnnotationInstance{
		Title: fmt.Sprintf("%s %v", pair.Key, pair.Values),
		Text:  fmt.Sprintf("%.2f %d", pair.Probability, pair.Sample),
		Time:  cointime.ToMilli(pair.Time),
		Tags:  []string{pair.Type.String(), pair.Strategy.Name},
	}
}

func StrategyEvent(event trade.StrategyEvent) model.AnnotationInstance {
	tag := ""
	if event.Result.Confidence > 0 {
		tag = "trigger"
	}
	return model.AnnotationInstance{
		Title: fmt.Sprintf("%s | %.2f/%.2f",
			event.Strategy,
			event.Result.Sum,
			event.Result.Count),
		Text: fmt.Sprintf("sample:%v , probability:%v , rating:%v\n%s\n%s",
			event.Sample.Predictions,
			event.Probability.Predictions,
			event.Result.Rating,
			emoji.PredictionValues(event.Probability.Values),
			joinSequences(event.Probability.Values)),
		Time: cointime.ToMilli(event.Time),
		Tags: []string{tag, emoji.MapType(event.Result.Type)},
	}
}

func joinSequences(sequences []buffer.Sequence) string {
	sqs := make([]string, len(sequences))
	for i, seq := range sequences {
		sqs[i] = string(seq)
	}
	return strings.Join(sqs, "|")
}
