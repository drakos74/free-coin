package position

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/model"
)

func formatOpenPositions(positions map[string]*model.Position) string {
	return fmt.Sprintf("open positions: %d", len(positions))
}

type position struct {
	t       time.Time
	coin    model.Coin
	open    float64
	current float64
	value   float64
	diff    float64
	ratio   float64
}

type positions []position

func (p positions) Len() int           { return len(p) }
func (p positions) Less(i, j int) bool { return p[i].t.Before(p[j].t) }
func (p positions) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func formatPosition(p position) string {
	return fmt.Sprintf("[%s] %08.2f -> %08.2f = %06.2f | %08.2f (%08.4f)", p.coin, p.open, p.current, p.diff, p.value, p.ratio)
}

func formatPositions(pp []position) string {
	msgs := new(strings.Builder)
	total := 0.0
	sort.Sort(positions(pp))
	for _, p := range pp {
		total += p.value
		msgs.WriteString(fmt.Sprintf("%s%s", formatPosition(p), "\n"))
	}
	msgs.WriteString(fmt.Sprintf("total : %.2f", total))
	return msgs.String()
}
