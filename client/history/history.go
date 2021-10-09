package history

import (
	"fmt"

	"github.com/drakos74/free-coin/client"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const (
	secondsInAnHour = 60 * 60
)

// History is a trade source implementation that intercepts the traffic and stores the trades,
// before forwarding them.
type History struct {
	source  client.Source
	batch   map[model.Coin][]*model.Trade
	index   map[model.Coin]string
	key     func(trade *model.Trade) string
	storage storage.Registry
}

// NewHistory creates a new history trade source implementation.
func NewHistory(source client.Source) *History {
	return &History{
		source:  source,
		batch:   make(map[model.Coin][]*model.Trade),
		index:   make(map[model.Coin]string),
		key:     genericKeyingFunc,
		storage: storage.NewVoidRegistry(),
	}
}

// Trades will delegate the trades the call to the underlying implementation,
// but intercepting and storing the traffic.
func (h *History) Trades(process <-chan api.Signal) (model.TradeSource, error) {
	// intercept the trades output channel
	out := make(model.TradeSource)
	in, err := h.source.Trades(process)
	if err != nil {
		return nil, fmt.Errorf("could not open trades channel: %w", err)
	}

	go func() {
		defer func() {
			close(out)
		}()
		for trade := range in {

			exchange := trade.Exchange
			coin := trade.Coin
			k := h.key(trade)

			key := Key{
				Coin:     coin,
				Exchange: exchange,
				Key:      k,
			}

			// assign trade to key cache
			err := h.storage.Add(storage.K{
				Pair:  string(coin),
				Label: key.String(),
			}, trade)
			if err != nil {
				log.Error().
					Str("coin", string(coin)).
					Str("exchange", exchange).
					Str("key", k).
					Err(err).Msg("could not store trade")
			}
			out <- trade
		}
	}()

	return out, nil
}

// Key defines the grouping key for the trades history.
type Key struct {
	Coin     model.Coin
	Exchange string
	Key      string
}

func (k Key) String() string {
	return fmt.Sprintf("%s_%s_%s", k.Exchange, k.Coin, k.Key)
}

func genericKeyingFunc(trade *model.Trade) string {
	// key on 4h period
	unixSeconds := trade.Time.Unix()
	hash := unixSeconds / (4 * secondsInAnHour)
	return fmt.Sprintf("%d", hash)
}