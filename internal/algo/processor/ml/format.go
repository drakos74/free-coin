package ml

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	model2 "github.com/drakos74/free-coin/internal/algo/processor/ml/model"

	"github.com/drakos74/free-coin/client"
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

func formatConfig(config model2.Config) string {

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

func formatReport(report client.Report) string {
	return fmt.Sprintf("[ buy : %d , sell : %d ] %.2f ( %.2f | %.2f ) ",
		report.Buy,
		report.Sell,
		report.Profit,
		report.Wallet,
		report.Fees)
}

func formatAction(action trader.Event, trend map[time.Duration]model.Trend, err error, ok bool) string {
	return fmt.Sprintf("%s\n%s|%.fm %s %.2f \n%s %.4f %s\n%.2f %s|%.2f|%.2f | %s\n%v|%v\n%+v",
		formatTime(action.Time),
		action.Key.Coin,
		action.Key.Duration.Minutes(),
		emoji.MapType(action.Type),

		action.Price,
		emoji.MapToSign(action.Value),
		action.Value,
		model.EURO,
		100*action.PnL,
		"%",
		action.CoinPnL,
		action.GlobalPnL,

		action.Reason,
		emoji.MapToAction(ok),
		err,
		formatTimeTrend(trend))
}

func formatSignal(signal model2.Signal, action trader.Event, err error, ok bool) string {
	return fmt.Sprintf("%s\n%s|%.fm|%s|%.2f %s\n%.4f %s %.2f%s\n%.2f%s|%.2f|%.2f %s (%.2f)\n%v|%v",
		formatTime(signal.Time),

		signal.Key.Coin,
		signal.Key.Duration.Minutes(),
		signal.Detail,
		signal.Trend,
		emoji.MapType(signal.Type),

		signal.Price,
		emoji.MapToSign(action.Value),
		action.Value,
		model.EURO,
		100*action.PnL,
		"%",
		action.CoinPnL,
		action.GlobalPnL,

		action.Reason,
		signal.Precision,
		emoji.MapToAction(ok),
		err)
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

		txtBuffer.WriteString(fmt.Sprintf("%.3f %s %s",
			value/t.State.OpenPrice,
			emoji.MapToSign(value),
			emoji.MapType(t.State.Type),
		))
		txtBuffer.WriteString(fmt.Sprintf("%+v [%s %s]",
			t.CurrentValue,
			emoji.MapType(t.Type[0]),
			emoji.MapType(t.Type[1]),
		))
	}
	return txtBuffer.String()
}

func encodeMessage(signal model2.Signal) string {
	bb, _ := json.Marshal(signal)
	return fmt.Sprintf("%s", string(bb))
}
