package ml

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/client"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/emoji"
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
	return fmt.Sprintf("[%s] %.2f [%d:%d]\n", c, stats.PnL, stats.Profit, stats.Loss)
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
	return fmt.Sprintf("%s : %.2f (%s %.2f%s) | %s (%.0f)",
		emoji.MapType(p.Type),
		p.OpenPrice*p.Volume,
		emoji.MapToSign(p.PnL),
		100*p.PnL,
		"%",
		p.CurrentTime.Format(time.Stamp), cointime.ToNow(p.CurrentTime),
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

func formatTrendReport(log bool, k model.Key, report trader.TrendReport) string {
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
			"%s|%.fm %s %s %.2f \n"+
			"%s %.4f %s\n"+
			"%.2f %s\n"+
			"%.2f[%d:%d]|%.2f[%d:%d]|%.2f[%d:%d] %s\n"+
			"%v|%v\n"+
			"%+v",
			formatTime(action.Time),
			action.Key.Coin,
			action.Key.Duration.Minutes(),
			action.Network,
			emoji.MapType(action.Type),

			action.Price,
			emoji.MapToSign(action.Value),
			action.Value,
			model.EURO,
			100*action.PnL,
			"%",
			action.TradeTracker.Network.PnL,
			action.TradeTracker.Network.Profit,
			action.TradeTracker.Network.Loss,
			action.Coin.PnL,
			action.Coin.Profit,
			action.Coin.Loss,
			action.Global.PnL,
			action.Global.Profit,
			action.Global.Loss,

			action.Reason,
			emoji.MapToAction(ok),
			err,
			formatTimeTrend(trend))
	}
	return fmt.Sprintf("(%.0f) %s||%s\n"+
		"%s %.2f%s\n"+
		"%s () %v",

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
			"%s|%.fm|%s-%d|%.2f %s\n"+
			"%.4f %s %.2f%s\n"+
			"%.2f%s\n"+
			"%.2f[%d:%d]|%.2f[%d:%d]|%.2f[%d:%d] %s (%.2f|%.2f)\n"+
			"%v|%v",
			formatTime(signal.Time),

			signal.Key.Coin,
			signal.Key.Duration.Minutes(),
			signal.Detail.Type,
			signal.Detail.Index,
			signal.Trend,
			emoji.MapType(signal.Type),

			signal.Price,
			emoji.MapToSign(action.Value),
			action.Value,
			model.EURO,
			100*action.PnL,
			"%",
			action.TradeTracker.Network.PnL,
			action.TradeTracker.Network.Profit,
			action.TradeTracker.Network.Loss,
			action.Coin.PnL,
			action.Coin.Profit,
			action.Coin.Loss,
			action.Global.PnL,
			action.Global.Profit,
			action.Global.Loss,

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
	return fmt.Sprintf("(%.0f) %s|%.2f|%s\n"+
		"%s %.2f%s\n"+
		"%s (%.2f|%.2f) %v",

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
		txtBuffer.WriteString(fmt.Sprintf("%+v\n", k))
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

func encodeMessage(signal mlmodel.Signal) string {
	bb, _ := json.Marshal(signal)
	return fmt.Sprintf("%s", string(bb))
}
