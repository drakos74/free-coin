package local

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

const timeFormat = "2006_01_02_15"

// Client will retrieve trades from the given key value storage.
// It will be able to retrieve and store data from an upstream client,
// if the storage does not contain the requested data.
// furthermore it will mock the client behaviour in terms of the positions and orders locally.
// It can be used as a simulation environment for testing processors and business logic.
type Client struct {
	since       int64
	upstream    func(since int64) (api.Client, error)
	persistence func(shard string) (storage.Persistence, error)
	trades      model.TradeSource
	hash        cointime.Hash
}

// NewClient creates a new client for trade processing.
func NewClient(ctx context.Context, since int64) *Client {
	return &Client{
		since:  since,
		hash:   cointime.NewHash(8 * time.Hour),
		trades: make(chan *model.Trade),
		upstream: func(since int64) (api.Client, error) {
			return Void(), nil
		},
		persistence: func(shard string) (storage.Persistence, error) {
			return storage.Void(), nil
		},
	}
}

// WithUpstream adds an upstream client to the local client.
func (c *Client) WithUpstream(upstream func(since int64) (api.Client, error)) *Client {
	c.upstream = upstream
	return c
}

// WithPersistence adds a storage layer to the local client.
func (c *Client) WithPersistence(persistence func(shard string) (storage.Persistence, error)) *Client {
	c.persistence = persistence
	return c
}

// Trades returns the trades starting at since.
// it will try to get trades from the persistence layer if available by day
// otherwise it will call the upstream client if available.
// In that sense it works always in batches of one day interval.
func (c *Client) Trades(stop <-chan struct{}, coin model.Coin, stopExecution api.Condition) (model.TradeSource, error) {

	// check if we have trades in the store ...
	start := cointime.FromNano(c.since)

	h := c.hash.Do(start)

	go func() {
		hash := h
		for {
			select {
			case <-stop:
				// we need to close our processing
				log.Info().Msg("closing local client")
				return
			default:
				nextHash, err := c.consumeBatch(hash, coin)
				if nextHash == hash || err != nil {
					log.Error().Err(err).Msg("error during batch processing")
					return
				}
				hash = nextHash
			}
		}
	}()

	return c.trades, nil

}

func (c *Client) consumeBatch(h int64, coin model.Coin) (int64, error) {
	startTime := c.hash.Undo(h)

	k := c.key(h, coin)
	store, err := c.persistence(string(coin))
	if err != nil {
		return h, err
	}

	trades := make([]model.Trade, 0)
	err = store.Load(k, &trades)
	log.Info().Err(err).Int64("hash", k.Hash).Time("from", startTime).Str("coin", string(coin)).Msg("loaded trades from local persistence")

	if err != nil {
		return h + 1, c.serveTradesFromUpstream(h, coin, store, err)
	}

	for _, trade := range trades {
		// add the meta map, which is ignored in the json
		trade.Meta = make(map[string]interface{})
		trade.Live = false
		trade.Active = false
		c.trades <- &trade
	}

	return h + 1, nil
}

func (c *Client) key(h int64, coin model.Coin) storage.Key {
	return storage.Key{
		Hash: h,
		Pair: string(coin),
		// TODO : make a method for this
		Label: fmt.Sprintf("from_%s_to_%s", c.hash.Undo(h).Format(timeFormat), c.hash.Undo(h+1).Format(timeFormat)),
	}
}

func (c *Client) serveTradesFromUpstream(h int64, coin model.Coin, store storage.Persistence, err error) error {
	startTime := c.hash.Undo(h)

	// any other error we ll effectively overwrite
	if errors.Is(err, storage.UnrecoverableErr) {
		log.Error().Err(err).Msg("initialise persistence")
		return err
	}

	// we need to load this batch from the upstream
	cl, err := c.upstream(startTime.UnixNano())
	if err != nil {
		log.Error().Err(err).Msg("could not create upstream")
		return err
	}
	stop := make(chan struct{}, 10)
	source, err := cl.Trades(stop, coin, api.NonStop)
	if err != nil {
		log.Error().Err(err).Msg("could not get trades from upstream")
		return err
	}
	k := c.key(h, coin)
	trades := make([]model.Trade, 0)
	var from *time.Time
	var to *time.Time
	for trade := range source {
		if from == nil {
			from = &trade.Time
		}
		to = &trade.Time
		// get the trades hash to see if it still belongs to our key
		hash := c.hash.Do(trade.Time)
		if hash == h {
			trades = append(trades, *trade)
			c.trades <- trade
		} else if hash > h {
			// the case where we got the first trade that is bigger than what we intended to get
			// we should have stopped now, because the channel should have closed.
			err = store.Store(k, trades)
			if err != nil {
				log.Error().Err(err).Msg("could not store trades")
				return err
			}
			log.Info().Time("from", *from).Time("to", *to).Err(err).Int64("hash", h).Msg("storing trade batch")
			h = hash
			k = c.key(h, coin)
			trades = make([]model.Trade, 0)
			from = nil
			to = nil
		}
	}
	return nil
}
