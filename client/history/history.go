package history

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	source   client.Source
	batch    map[model.Coin][]*model.Trade
	index    map[model.Coin]string
	key      func(t time.Time) string
	deKey    func(t string) (time.Time, error)
	registry storage.Registry
	readonly *Request
}

// New creates a new history trade source implementation.
func New(source client.Source) *History {
	return &History{
		source:   source,
		batch:    make(map[model.Coin][]*model.Trade),
		index:    make(map[model.Coin]string),
		key:      genericKeyingFunc,
		deKey:    genericDeKeyFunc,
		registry: storage.NewVoidRegistry(),
	}
}

// WithRegistry defines the registry for the registry
func (h *History) WithRegistry(registry storage.Registry) *History {
	h.registry = registry
	return h
}

func (h *History) Reader(request *Request) *History {
	h.readonly = request
	return h
}

type report struct {
	init     bool
	start    time.Time
	startGap time.Duration
	end      time.Time
	endGap   time.Duration
}

func (r report) String() string {
	return fmt.Sprintf("init = %v , start = %+v | %+v , end = %+v | %+v",
		r.init, r.start.String(), r.startGap.Hours()/24, r.end.String(), r.endGap.Hours()/24)
}

type Range struct {
	Path string
	Hash int
	From time.Time
	To   time.Time
}

func (h *History) Ranges(coin model.Coin, from, to time.Time) []Range {
	paths, err := h.registry.Check(storage.K{
		Pair: string(coin),
	})
	if err != nil {
		log.Error().Err(err).Str("coin", string(coin)).Msg("could not check registry")
	}

	ranges := make([]Range, 0)
	for i := 0; i < len(paths); i++ {

		label := paths[i]

		if len(label) == 0 {
			continue
		}

		parts := strings.Split(label, "_")
		if len(parts) != 3 {
			log.Warn().Str("label", label).Strs("key", parts).Err(err).Msg("could not parse label as key")
			continue
		}
		t := parts[2]

		hash, err := strconv.Atoi(t)
		if err != nil {
			log.Warn().Str("label", label).Str("time", t).Err(err).Msg("could not parse label as hash")
			continue
		}

		tt, err := h.deKey(t)
		if err != nil {
			log.Warn().Str("label", label).Str("time", t).Err(err).Msg("could not parse label as time")
			continue
		}

		if tt.Before(from) || tt.After(to) {
			continue
		}

		ranges = append(ranges, Range{
			Path: paths[i],
			Hash: hash,
			From: tt,
		})
	}
	return ranges
}

// Trades will delegate the trades the call to the underlying implementation,
// but intercepting and storing the traffic.
func (h *History) Trades(process <-chan api.Signal) (model.TradeSource, error) {
	// intercept the trades output channel
	out := make(model.TradeSource)
	in := make(model.TradeSource)
	if h.readonly != nil {
		audit := report{}
		inCh, err := newSource(*h.readonly, h.registry).
			WithFilter(func(label string) bool {
				// split the label
				parts := strings.Split(label, "_")
				if len(parts) != 3 {
					return true
				}
				t := parts[2]

				tt, err := h.deKey(t)
				if err != nil {
					log.Warn().Str("label", label).Str("time", t).Err(err).Msg("could not parse label as time")
					return true
				}
				if tt.Before(h.readonly.From.Add(-1*time.Hour)) || tt.After(h.readonly.To.Add(5*time.Hour)) {
					return false
				}

				if !audit.init {
					audit.init = true
					audit.start = tt
				}
				audit.end = tt
				return true
			}).Trades(process)
		audit.startGap = audit.start.Sub(h.readonly.From)
		audit.endGap = audit.end.Sub(h.readonly.To)
		fmt.Printf("report = %+v\n", audit)
		fmt.Printf("h.readOnly = %+v\n", h.readonly)

		if err != nil {
			return nil, fmt.Errorf("could not creste readonly source: %w", err)
		}
		in = inCh
	} else {
		inCh, err := h.source.Trades(process)
		if err != nil {
			return nil, fmt.Errorf("could not open trades channel: %w", err)
		}
		in = inCh
	}

	go func() {
		defer func() {
			log.Info().Str("processor", "trades-history").Msg("closing processor")
			close(out)
		}()

		kk := ""

		for trade := range in {
			if h.readonly == nil {
				// store the trade
				exchange := trade.Exchange
				coin := trade.Coin
				k := h.key(trade.Time)

				// structure : {Exchange}_{Coin}_{time-hash}
				key := Key{
					Coin:     coin,
					Exchange: exchange,
					Key:      k,
				}

				if kk != k {
					fmt.Printf("trade.Time = %+v | %+v\n", trade.Time, key)
					kk = k
				}

				// assign trade to key cache
				err := h.registry.Add(storage.K{
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

func genericKeyingFunc(t time.Time) string {
	// key on 4h period
	unixSeconds := t.Unix()
	hash := unixSeconds / (4 * secondsInAnHour)
	return fmt.Sprintf("%d", hash)
}

func genericDeKeyFunc(t string) (time.Time, error) {
	h, err := strconv.Atoi(t)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse number from %s: %w", t, err)
	}
	seconds := h * 4 * secondsInAnHour
	return time.Unix(int64(seconds), 0), nil
}
