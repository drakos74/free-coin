package ml

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/client"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/drakos74/free-coin/internal/trader"
)

func formatSettings(settings trader.Settings) string {
	return fmt.Sprintf("%.2f [take-profit = %.3f | stop-loss = %.3f]",
		settings.OpenValue,
		settings.TakeProfit,
		settings.StopLoss)
}

func formatStat(c string, stats trader.Stats) string {
	return fmt.Sprintf("[%s] %.2f(%.2f) [%d:%d]\n", c, stats.PnL, stats.Value, stats.Profit, stats.Loss)
}

func formatConfig(config mlmodel.Config) string {

	buffer := new(strings.Builder)

	for k, segment := range config.Segments {
		buffer.WriteString(fmt.Sprintf("%+v\n%s %s - %s\n",
			k.ToString(),
			segment.Model.Format(),
			segment.Stats.Format(), segment.Trader.Format()))
	}

	return fmt.Sprintf("%d\n%s (%.2f€ +%.2f -%.2f) \n[debug=%v,benchmark=%v]",
		len(config.Segments),
		buffer.String(),
		config.Position.OpenValue,
		config.Position.TakeProfit,
		config.Position.StopLoss,
		config.Option.Debug,
		config.Option.Benchmark,
	)
}
func formatPosition(p model.Position) string {

	// gather the trends
	buffer := new(strings.Builder)
	for k, tr := range p.Trend {
		buffer.WriteString(fmt.Sprintf("%v:value:%+v:type=%+v:shift=%+v", k, tr.CurrentValue, tr.Type, tr.Shift))
	}

	return fmt.Sprintf("%s : %.2f (%s %.2f%s) | %s (%.0f)"+
		"\n%s",
		emoji.MapType(p.Type),
		p.OpenPrice*p.Volume,
		emoji.MapToSign(p.PnL),
		100*p.PnL,
		"%",
		p.CurrentTime.Format(time.Stamp), cointime.ToNow(p.CurrentTime),
		buffer.String(),
	)
}

func formatKey(k model.Key) string {
	return fmt.Sprintf("%s|%.0f",
		k.Coin,
		k.Duration.Minutes())
}

func formatReport(report client.Report) string {
	return fmt.Sprintf("[ buy : %d , sell : %d ] %.2f ( %.2f | %.2f ) ",
		report.Buy,
		report.Sell,
		report.Profit,
		report.Wallet,
		report.Fees)
}

func formatTrendReport(log bool, k model.Key, report model.TrendReport) string {
	return fmt.Sprintf("%s %.2f (loss=%v , profit=%v)\n"+
		"[%s , %s]",
		formatKey(k),
		report.Profit,
		report.StopLossActive,
		report.TakeProfitActive,
		emoji.MapType(report.ValidTrend[0]), emoji.MapType(report.ValidTrend[1]),
	)
}

func formatAction(log bool, action trader.Event, trend map[time.Duration]model.Trend, err error, ok bool) string {
	if log {
		return fmt.Sprintf("%s\n"+
			"%s|%.fm|%s|%s %.2f\n"+
			"%s %s %.2f%s %.2f%s\n"+
			"%s%s%s%s\n"+
			"%v|%v\n"+
			"%+v",
			formatTime(action.Time),

			action.Key.Coin,
			action.Key.Duration.Minutes(),
			action.Key.Strategy,
			action.Key.Network,
			action.Price,

			emoji.MapType(action.Type),
			emoji.MapToSign(action.Value),
			100*action.PnL, "%",
			action.Value, model.EURO,

			formatStat("", action.TradeTracker.Network),
			formatStat("", action.Coin),
			formatStat("", action.Global),
			action.Reason,

			emoji.MapToAction(ok),
			err,
			formatTimeTrend(trend))
	}
	return fmt.Sprintf("(%.0f) %s %s %s\n"+
		"%.2f%s %s %v",

		cointime.ToNow(action.Time),

		emoji.MapType(action.Type),
		action.Key.Coin,

		emoji.MapToSign(action.Value),
		100*action.PnL,
		"%",

		action.Reason,
		emoji.MapToAction(ok))
}

func formatSignal(log bool, signal mlmodel.Signal, action trader.Event, err error, ok bool) string {
	if log {
		return fmt.Sprintf("%s\n"+
			"%s|%.fm|%s-%d|%.2f %.4f\n"+
			"%s %s %.2f%s %.2f%s\n"+
			"%s%s%s%s (%.2f|%.2f)\n"+
			"%v|%v",

			// time
			formatTime(signal.Time),

			// coin / strategy
			signal.Key.Coin,
			signal.Key.Duration.Minutes(),
			signal.Detail.Type,
			signal.Detail.Index,
			signal.Trend,
			signal.Price,

			// coin / position data
			emoji.MapType(signal.Type),
			emoji.MapToSign(action.Value),
			100*action.PnL, "%",
			action.Value,
			model.EURO,

			// stats
			formatStat("", action.TradeTracker.Network),
			formatStat("", action.Coin),
			formatStat("", action.Global),

			// action details
			action.Reason,
			signal.Precision,
			signal.Gap,

			emoji.MapToAction(ok),
			err)

		//Jan 22 10:17:06 (121)
		//SOL|0m|net.RandomForest-0|7.00 🤑
		//23.1265 🔥 1.23€
		//0.40%
		//	5.87[14:8]|7.53[5:0]|5.87[14:8] signal (0.24)
		//♻|<nil>
		//🚛
	}
	return fmt.Sprintf("(%.0f) %s|%.2f|%s %s\n"+
		"%.2f%s %s (%.2f|%.2f) %v",

		cointime.ToNow(signal.Time),

		emoji.MapType(signal.Type),
		signal.Trend,
		signal.Key.Coin,

		emoji.MapToSign(action.Value),
		100*action.PnL,
		"%",

		action.Reason,
		signal.Precision,
		signal.Gap,
		emoji.MapToAction(ok))
}

func formatTime(t time.Time) string {
	return fmt.Sprintf("%s (%.0f)",
		t.Format(time.Stamp), cointime.ToNow(t),
	)
}

func formatTrend(trend map[model.Key]map[time.Duration]model.Trend) string {
	txtBuffer := new(strings.Builder)
	for k, tt := range trend {
		txtBuffer.WriteString(fmt.Sprintf("%s ", k.Coin))
		txtBuffer.WriteString(formatTimeTrend(tt))
	}
	return txtBuffer.String()
}

func formatTimeTrend(tt map[time.Duration]model.Trend) string {
	txtBuffer := new(strings.Builder)
	for _, t := range tt {
		value := 0.0
		switch t.State.Type {
		case model.Buy:
			value = t.State.CurrentPrice - t.State.OpenPrice
		case model.Sell:
			value = t.State.OpenPrice - t.State.CurrentPrice
		}

		txtBuffer.WriteString(fmt.Sprintf("%.3f %s %s ",
			value/t.State.OpenPrice,
			emoji.MapToSign(value),
			emoji.MapType(t.State.Type),
		))
		txtBuffer.WriteString(fmt.Sprintf("[%s %s]",
			emoji.MapToTrend(t.Type[0]),
			emoji.MapToTrend(t.Type[1]),
		))
	}
	return txtBuffer.String()
}

func formatDecision(decision *model.Decision) string {
	if decision == nil {
		return ""
	}
	return fmt.Sprintf("%.2f %+v\n"+
		"%+v\n"+
		"%+v\v",
		decision.Confidence, formatFloats(decision.Config),
		formatFloats(decision.Features),
		formatFloats(decision.Importance))
}

func formatPrediction(cluster int, score float64, metadata ml.Metadata, err error) string {
	s := new(strings.Builder)
	for g, st := range metadata.Stats {
		s.WriteString(fmt.Sprintf("%d (%d | %.2f)\n", g, st.Size, st.Avg))
	}
	return fmt.Sprintf("%.d | %.4f | %s|%d \n %s", cluster, score, fmt.Errorf("err = %w", err).Error(), metadata.Samples, s.String())
}

func formatSpectrum(spectrum math.Spectrum) string {
	return fmt.Sprintf("%.2f (%.2f) %s", spectrum.Amplitude, spectrum.Mean(), formatRNums(spectrum.Values))
}

func formatFloats(ff []float64) string {
	s := new(strings.Builder)
	s.WriteString("[")
	for i := 0; i < len(ff); i++ {
		if i != 0 {
			s.WriteString(",")
		}
		s.WriteString(fmt.Sprintf(" %.2f", ff[i]))
	}
	s.WriteString(" ]")
	return s.String()
}

func formatRNums(ff []math.RNum) string {
	s := new(strings.Builder)
	s.WriteString("[")
	for i := 0; i < len(ff); i++ {
		if i != 0 {
			s.WriteString(",")
		}
		s.WriteString(fmt.Sprintf(" %d|%.2f ", ff[i].Frequency, ff[i].Amplitude))
	}
	s.WriteString(" ]")
	return s.String()
}

func encodeMessage(signal mlmodel.Signal) string {
	bb, _ := json.Marshal(signal)
	return fmt.Sprintf("%s", string(bb))
}
