package stats

import (
	"fmt"
	"math"
	"strings"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/emoji"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
)

func formatΗΜΜMessage(last windowView, values [][]string, aggregateStats coinmath.AggregateStats, predictions map[buffer.Sequence]buffer.Predictions, status buffer.Status, p *model.Trade, cfg Config) string {
	// TODO : make the trigger arguments more specific to current stats statsCollector

	// format the PredictionList.
	pp := make([]string, len(predictions)+1)
	samples := make([]string, len(status.Samples))
	k := 0
	for _, sample := range status.Samples {
		var ss int
		for _, smpl := range sample {
			ss += smpl.Events
		}
		samples[k] = fmt.Sprintf("%d : %d", ss, len(sample))
		k++
	}
	pp[0] = fmt.Sprintf("%+v -> %s", status.Count, strings.Join(samples, " | "))
	i := 1
	for _, v := range predictions {
		pp[i] = fmt.Sprintf("%s -> %v [ %d : %d : %d ] ",
			emoji.Sequence(v.Key),
			emoji.PredictionList(v.Values),
			len(v.Values),
			v.Sample,
			v.Count,
		)
		i++
	}

	// format the past values
	emojiValues := make([]string, len(values[0]))
	for j := 0; j < len(values[0]); j++ {
		emojiValues[j] = emoji.MapToSymbol(values[0][j])
	}

	// stats processor details
	ps := fmt.Sprintf("%s|%dm: %s ...",
		p.Coin,
		cfg.Duration,
		strings.Join(emojiValues, " "))

	// last bucket Price details
	move := emoji.MapToSentiment(last.price.Ratio)
	mp := fmt.Sprintf("%s %s€ ratio:%.2f stdv:%.2f ema:%.2f",
		move,
		coinmath.Format(p.Price),
		last.price.Ratio*100,
		last.price.StdDev,
		last.price.EMADiff)

	// ignore the values smaller than '0' just to be certain
	vol := emoji.MapNumber(coinmath.O2(math.Round(last.volume.Diff)))
	mv := fmt.Sprintf("%s %s€ ratio:%.2f stdv:%.2f ema:%.2f",
		vol,
		coinmath.Format(last.volume.Value),
		last.volume.Ratio,
		last.volume.StdDev,
		last.volume.EMADiff)

	// bucket collector details
	st := fmt.Sprintf("rsi:%d ersi:%d ema:%.2f (%d)",
		aggregateStats.RSI,
		aggregateStats.ERSI,
		100*(aggregateStats.EMA-last.price.Value)/last.price.Value,
		aggregateStats.Sample)

	// TODO : make this formatting easier
	// format the status message for the processor.
	return fmt.Sprintf("%s\n %s\n %s\n %s\n %s",
		ps,
		mp,
		mv,
		st,
		// PredictionList details
		strings.Join(pp, "\n "))

}

func formatPoly(cfg Config, trade *model.Trade, poly map[int][]float64, density float64, profit float64, err error) string {
	return fmt.Sprintf("%s | %s %s (%.2f %.2f) | %.2f | %.2f (%.2f) \n%v %v %v",
		trade.Coin,
		emoji.MapDeca(poly[2][2]), emoji.MapDeca(100*poly[3][3]),
		poly[2][2], poly[3][3],
		trade.Price, density, profit,
		cfg.Name, cfg.Duration, cfg.Model.Stats)
}

func formatSignal(signal Signal, threshold int) string {
	factor := math.Pow(10, float64(threshold)) * signal.Factor
	if signal.Type == model.Sell {
		factor = -1 * factor
	}
	return fmt.Sprintf("%s %s %s %.4f | %.2f | (%+v|%d) ", signal.Coin, emoji.MapType(signal.Type), emoji.MapDeca(factor), signal.Price, signal.Density, signal.Duration, signal.Segments)
}
