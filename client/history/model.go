package history

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/model"
)

type Request struct {
	Coin model.Coin
	From time.Time
	To   time.Time
}

func (r Request) String() string {
	return fmt.Sprintf("coin = %v , from = %v , to = %v", r.Coin, r.From, r.To)
}
