package history

import (
	"time"

	"github.com/drakos74/free-coin/internal/model"
)

type Request struct {
	Coin model.Coin
	From time.Time
	To   time.Time
}
