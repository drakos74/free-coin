package ml

import (
	"encoding/json"
	"fmt"

	"github.com/drakos74/free-coin/internal/emoji"
)

func formatMessage(signal Signal) string {
	return fmt.Sprintf("%s:%.fm %s (%.4f) | %.2f",
		signal.Coin,
		signal.Duration.Minutes(),
		emoji.MapType(signal.Type),
		signal.Price,
		signal.Precision,
	)
}

func encodeMessage(signal Signal) string {
	bb, _ := json.Marshal(signal)
	return fmt.Sprintf("%s", string(bb))
}
