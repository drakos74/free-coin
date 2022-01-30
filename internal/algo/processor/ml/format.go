package ml

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/trader"

	"github.com/drakos74/free-coin/client"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
)

func formatPosition(p model.Position) string {
	return fmt.Sprintf("%s : %.2f|%.2f (%.2f)",
		emoji.MapType(p.Type),
		p.OpenPrice,
		p.Volume,
		p.PnL,
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

func formatAction(action trader.Action, err error, ok bool) string {
	return fmt.Sprintf("%s - %.2f | %s\n%v|err=%s", action.Key.ToString(), action.Value, action.Reason, ok, err)
}

func formatSignal(signal Signal) string {
	return fmt.Sprintf("%s | %s:%.fm %s (%.4f) | %.2f",
		signal.Time.Format(time.Stamp),
		signal.Coin,
		signal.Duration.Minutes(),
		emoji.MapType(signal.Type),
		signal.Price,
		signal.Precision)
}

func encodeMessage(signal Signal) string {
	bb, _ := json.Marshal(signal)
	return fmt.Sprintf("%s", string(bb))
}

func formatSignals(signals map[time.Duration]Signal) string {
	msg := new(strings.Builder)
	for _, signal := range signals {
		msg.WriteString(fmt.Sprintf("%s\n", formatSignal(signal)))
	}
	return msg.String()
}