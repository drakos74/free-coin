package position

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
)

func formatOpenPositions(positions map[string]*model.Position) string {
	return fmt.Sprintf("open positions: %d", len(positions))
}

type exchangePosition struct {
	model.Position
	depth int
}

type position struct {
	t       time.Time
	coin    model.Coin
	open    float64
	current float64
	value   float64
	diff    float64
	ratio   ratio
}

type positions []position

func (p positions) Len() int           { return len(p) }
func (p positions) Less(i, j int) bool { return p[i].t.Before(p[j].t) }
func (p positions) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type ratio map[time.Duration]float64

func (r ratio) hasValue() bool {
	for _, v := range r {
		if math.Abs(v) > 0.001 {
			return true
		}
	}
	return false
}

func (r ratio) String() string {
	msgs := new(strings.Builder)
	msgs.WriteString("(")
	for k, p := range r {
		msgs.WriteString(fmt.Sprintf("%.0f:%.4f:%s", k.Minutes(), p, emoji.MapLog10(p)))
	}
	msgs.WriteString(")")
	return msgs.String()
}

func formatPosition(p position) string {
	return fmt.Sprintf("[%s] %08.2f -> %08.2f = %06.2f | %s %s", p.coin, p.open, p.current, p.diff, emoji.MapToSign(p.value), p.ratio)
}

func formatPositions(pp map[model.Coin][]position) (string, string, bool) {
	msgs := new(strings.Builder)
	total := 0.0
	hasLines := false

	coins := make([]model.Coin, 0)
	positions := make(map[model.Coin]position, 0)
	for c, pps := range pp {
		coins = append(coins, c)
		pos := position{}
		// TODO : NOTE !!! the aggregation logic assumes all positions for the same coin being of equal size
		for _, p := range pps {
			pos.coin = p.coin
			pos.diff += p.diff
			pos.current = p.current
			pos.open += p.open
			pos.ratio = p.ratio
		}
		pos.open = pos.open / float64(len(pps))
		pos.diff = pos.diff / float64(len(pps))
		positions[c] = pos
	}

	for _, c := range coins {
		p := positions[c]
		total += p.value
		//if p.ratio.hasValue() {
		msgs.WriteString(fmt.Sprintf("%s%s", formatPosition(p), "\n"))
		hasLines = true
		//}
	}
	v := emoji.ConvertValue(total)
	msgs.WriteString(fmt.Sprintf("total => %.2f %s", total, emoji.ConvertValue(total)))
	return msgs.String(), v, hasLines
}
