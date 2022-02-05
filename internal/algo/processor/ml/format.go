package ml

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/drakos74/free-coin/client"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/trader"
)

func formatSettings(settings trader.Settings) string {
	return fmt.Sprintf("%.2f [take-profit = %.2f | stop-loss = %.2f]",
		settings.OpenValue,
		settings.TakeProfit,
		settings.StopLoss)
}

func formatConfig(config Config) string {
	return fmt.Sprintf("%d (%.2f€ +%.2f -%.2f) \n[debug=%v,benchmark=%v]",
		len(config.Segments),
		config.Position.OpenValue,
		config.Position.TakeProfit,
		config.Position.StopLoss,
		config.Debug,
		config.Benchmark,
	)
}
func formatPosition(p model.Position) string {
	return fmt.Sprintf("%s : %.2f (%s %.2f%s) | %s",
		emoji.MapType(p.Type),
		p.OpenPrice*p.Volume,
		emoji.MapToSign(p.PnL),
		100*p.PnL,
		"%",
		p.CurrentTime.Format(time.Stamp),
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

func formatAction(action trader.Event, profit float64, err error, ok bool) string {
	return fmt.Sprintf("%s | %s:%.fm %s (%.4f|%s %.2f%s|%.2f%s) | %s\n%v|%v",
		action.Time.Format(time.Stamp),
		action.Key.Coin,
		action.Key.Duration.Minutes(),
		emoji.MapType(action.Type),
		action.Price,
		emoji.MapToSign(action.Value),
		100*profit,
		"%",
		100*action.PnL,
		"%",
		action.Reason,
		emoji.MapToAction(ok),
		err)
}

func formatSignal(signal Signal, value float64, profit float64, err error, ok bool) string {
	return fmt.Sprintf("%s | %s:%.fm %s (%.4f|%s %.2f%s) | %.2f\n%v|%v",
		signal.Time.Format(time.Stamp),
		signal.Coin,
		signal.Duration.Minutes(),
		emoji.MapType(signal.Type),
		signal.Price,
		emoji.MapToSign(value),
		100*profit,
		"%",
		signal.Precision,
		emoji.MapToAction(ok),
		err)
}

func encodeMessage(signal Signal) string {
	bb, _ := json.Marshal(signal)
	return fmt.Sprintf("%s", string(bb))
}
