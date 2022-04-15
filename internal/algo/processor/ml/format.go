package ml

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

func formatConfig(config Config) string {

	buffer := new(strings.Builder)

	for k, segment := range config.Segments {
		buffer.WriteString(fmt.Sprintf("%+v - %+v\n", k, segment))
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

func formatAction(action trader.Event, profit []float64, trend map[time.Duration]model.Trend, err error, ok bool) string {
	return fmt.Sprintf("%s (%.0f) | %s:%.fm %s (%.4f|%s|%.2f%s %.2f%s%.2f) | %s\n%v|%v\n%+v",
		action.Time.Format(time.Stamp), cointime.ToNow(action.Time),
		action.Key.Coin,
		action.Key.Duration.Minutes(),
		emoji.MapType(action.Type),
		action.Price,
		emoji.MapToSign(action.Value),
		action.Value,
		model.EURO,
		100*action.PnL,
		"%",
		profit,
		action.Reason,
		emoji.MapToAction(ok),
		err,
		trend)
}

func formatSignal(signal Signal, action trader.Event, err error, ok bool) string {
	return fmt.Sprintf("%s (%.0f) | %s:%.fm:%s %s (%.4f|%s|%.2f%s %.2f%s|%.2f|%.2f) | %s (%.2f)\n%v|%v",
		signal.Time.Format(time.Stamp), cointime.ToNow(signal.Time),
		signal.Key.Coin,
		signal.Key.Duration.Minutes(),
		signal.Detail,
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

func formatTrend(signal *model.TradeSignal, trend map[model.Key]map[time.Duration]model.Trend) string {
	txtBuffer := new(strings.Builder)
	txtBuffer.WriteString("\n")
	for k, tt := range trend {
		txtBuffer.WriteString(fmt.Sprintf("%+v\n", k))
		txtBuffer.WriteString(fmt.Sprintf("%s (%.0f) | [%s] %+v\n",
			signal.Tick.Time.Format(time.Stamp), cointime.ToNow(signal.Tick.Time),
			signal.Coin,
			tt))
	}
	return txtBuffer.String()
}

func encodeMessage(signal Signal) string {
	bb, _ := json.Marshal(signal)
	return fmt.Sprintf("%s", string(bb))
}
