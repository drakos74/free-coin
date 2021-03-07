package processor

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const Name = " void"

// Key characterises distinct processor attributes
type Key struct {
	Coin     model.Coin
	Duration time.Duration
	Index    int
}

func NewKey(c model.Coin, d time.Duration) Key {
	return Key{
		Coin:     c,
		Duration: d,
	}
}

func (k Key) String() string {
	return fmt.Sprintf("coin = %s , duration = %v", k.Coin, k.Duration)
}

func Void(name string) api.Processor {
	processorName := fmt.Sprintf("%s-%s", name, Name)
	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			log.Info().Str("processor", processorName).Msg("closing processor")
			close(out)
		}()
		for trade := range in {
			out <- trade
		}
	}
}

func Audit(name, msg string) string {
	return fmt.Sprintf("[%s] %s", name, msg)
}
