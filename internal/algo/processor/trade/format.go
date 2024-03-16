package trade

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
		buffer.WriteString(fmt.Sprintf("%+v %s\n",
			k.ToString(), segment.Trader.Format()))
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
	return fmt.Sprintf("%s : %.2f (%s %.2f%s) | %s (%.0f)"+
		"\n%s",
		emoji.MapType(p.Type),
		p.OpenPrice*p.Volume,
		emoji.MapToSign(p.PnL),
		100*p.PnL,
		"%",
		p.CurrentTime.Format(time.Stamp), cointime.ToNow(p.CurrentTime),
		formatPositionTrend(p.Trend),
	)
}

func formatPositionTrend(trend map[time.Duration]model.Trend) string {
	if len(trend) == 0 {
		return ""
	}
	buffer := new(strings.Builder)
	for k, tr := range trend {
		buffer.WriteString(formatTrendValue(k, tr))
	}
	return buffer.String()
}

func formatTrendValue(k time.Duration, trend model.Trend) string {
	return fmt.Sprintf("%v:%+v|%+v\n"+
		"%+v\n%+v\n"+
		"%+v\n%+v",
		k, formatTypes(emoji.ArrowUp, trend.Type, emoji.MapToTrend), formatTypes(emoji.ArrowCurve, trend.Shift, emoji.MapToTrend),
		formatFloats(emoji.Equal, trend.LastValue, func(f float64) string {
			return fmt.Sprintf(" %.4f", f)
		}),
		formatFloats(emoji.Minus, trend.CurrentValue, func(f float64) string {
			return fmt.Sprintf(" %.4f", f)
		}),
		formatFloats("x", trend.XX, func(f float64) string {
			return fmt.Sprintf(" %.2f", f)
		}), formatFloats("y", trend.YY, func(f float64) string {
			return fmt.Sprintf(" %.2f", f)
		}))
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
	return fmt.Sprintf("(%.0f|%.0f) %s\n"+
		"%s %v\n"+
		"%s%s"+
		"%s",

		cointime.ToNow(action.Time),
		cointime.ToNow(action.SourceTime),

		formatCoinPosition(action.Key.Coin, action.Type, 0, action.Value, action.PnL),

		action.Reason,
		emoji.MapToAction(ok),
		formatStat("", action.Coin),
		formatStat("", action.Global),
		formatPositionTrend(action.Trend),
	)
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
	return fmt.Sprintf("(%.0f) %s\n"+
		"%s (%.2f|%.2f) %v\n"+
		"%s%s"+
		"%s",

		cointime.ToNow(signal.Time),

		formatCoinPosition(signal.Key.Coin, signal.Type, signal.Trend, action.Value, action.PnL),

		action.Reason,
		signal.Precision,
		signal.Gap,
		emoji.MapToAction(ok),
		formatStat("", action.Coin),
		formatStat("", action.Global),
		formatPositionTrend(action.Trend),
	)
}

func formatCoinPosition(c model.Coin, t model.Type, trend float64, value float64, pnl float64) string {
	return fmt.Sprintf("%s|%.2f|%s %s %.2f%s",
		emoji.MapType(t),
		trend,
		c,
		emoji.MapToSign(value),
		100*pnl,
		"%",
	)
}

func formatTime(t time.Time) string {
	return fmt.Sprintf("%s (%.0f)",
		t.Format(time.Stamp), cointime.ToNow(t),
	)
}

func formatTrend(trend map[model.Key]map[time.Duration]model.Trend) string {
	txtBuffer := new(strings.Builder)
	for _, tt := range trend {
		//txtBuffer.WriteString(fmt.Sprintf("%s ", k.Coin))
		txtBuffer.WriteString(formatTimeTrend(tt))
	}
	return txtBuffer.String()
}

func formatTimeTrend(tt map[time.Duration]model.Trend) string {
	txtBuffer := new(strings.Builder)
	for k, t := range tt {
		txtBuffer.WriteString(fmt.Sprintf("%s\n", formatPositionState(t.State)))
		txtBuffer.WriteString(formatTrendValue(k, t))
	}
	return txtBuffer.String()
}

func formatPositionState(state model.PositionState) string {
	return formatCoinPosition(state.Coin, state.Type, 0, state.Value, state.PnL)
}

//func formatDecision(decision *model.Decision) string {
//	if decision == nil {
//		return ""
//	}
//	return fmt.Sprintf("%.2f %+v\n"+
//		"%+v\n"+
//		"%+v\v",
//		decision.Confidence, formatFloats(decision.Config, func(f float64) string {
//			return fmt.Sprintf(" %.2f", f)
//		}),
//		formatFloats(decision.Features, func(f float64) string {
//			return fmt.Sprintf(" %.2f", f)
//		}),
//		formatFloats(decision.Importance, func(f float64) string {
//			return fmt.Sprintf(" %.2f", f)
//		}))
//}

func formatPrediction(trace bool, cluster int, score float64, metadata ml.Metadata, err error) string {
	if trace {
		s := new(strings.Builder)
		for g, st := range metadata.Clusters {
			s.WriteString(fmt.Sprintf("%d (%d | %.2f)\n", g, st.Size, st.Avg))
		}
		return fmt.Sprintf("%.d | %.4f | %s|%d \n %s", cluster, score, fmt.Errorf("%w", err).Error(), metadata.Samples, s.String())
	} else if err != nil {
		return fmt.Sprintf("%.d | %.4f | %s|%d", cluster, score, fmt.Errorf("err = %w", err).Error(), metadata.Samples)
	}
	return fmt.Sprintf("%.d | %.4f (%.4f) | %d", cluster, score, metadata.Accuracy, metadata.Samples)
}

func formatSpectrum(spectrum math.Spectrum) string {
	return fmt.Sprintf("%.2f (%.2f) %s", spectrum.Amplitude, spectrum.Mean(), formatRNums(spectrum.Values))
}

func formatFloats(title string, ff []float64, format func(f float64) string) string {
	s := new(strings.Builder)
	s.WriteString(fmt.Sprintf("%s:[", title))
	for i := 0; i < len(ff); i++ {
		if i != 0 {
			s.WriteString(",")
		}
		s.WriteString(format(ff[i]))
	}
	s.WriteString(" ]")
	return s.String()
}

func formatTypes(title string, ff []model.Type, format func(i model.Type) string) string {
	s := new(strings.Builder)
	s.WriteString(fmt.Sprintf("%s:[", title))
	for i := 0; i < len(ff); i++ {
		if i != 0 {
			s.WriteString(",")
		}
		s.WriteString(format(ff[i]))
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
