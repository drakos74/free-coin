package local

import (
	"fmt"
	"time"

	"github.com/google/uuid"

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
	timeRange   cointime.Range
	uuid        string
	upstream    func(since int64) (api.Client, error)
	persistence func(shard string) (storage.Persistence, error)
	trades      map[model.Coin]model.TradeSource
	hash        cointime.Hash
	mock        bool
}

// NewClient creates a new client for trade processing.
func NewClient(timeRange cointime.Range, uuid string) *Client {
	return &Client{
		timeRange: timeRange,
		uuid:      uuid,
		hash:      cointime.NewHash(8 * time.Hour),
		trades:    make(map[model.Coin]model.TradeSource),
		upstream: func(since int64) (api.Client, error) {
			return Void(), nil
		},
		persistence: func(shard string) (storage.Persistence, error) {
			return storage.NewVoidStorage(), nil
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

// Mock will emulate all trades as active and live, so that processors can process them.
func (c *Client) Mock() *Client {
	c.mock = true
	return c
}

// Trades returns the trades starting at since.
// it will try to get trades from the persistence layer if available by day
// otherwise it will call the upstream client if available.
// In that sense it works always in batches of one day interval.
func (c *Client) Trades(process <-chan api.Action, query api.Query) (model.TradeSource, error) {
	// check if we have trades in the store ...

	if _, ok := c.trades[query.Coin]; !ok {
		c.trades[query.Coin] = make(chan *model.Trade)
	}
	// NOTE : we are making a major assumption here that timestamps will always increase.
	go func(timeRange cointime.Range, query api.Query) {

		store, err := c.persistence(string(query.Coin))
		if err != nil {
			log.Error().Err(err).Msg("could not initialise storage")
			// TODO : we need to signal back to the caller (maybe use stop channel ?)
			return
		}

		startTime := timeRange.From
		for {
			endTime, err := c.localTrades(query.Index, startTime, query.Coin, store, process)
			if err != nil {
				log.Warn().
					Err(err).
					Time("start-time", startTime).
					Msg("could not load more local trades")
				// get from upstream trades not found or local storage is not working
				break
			}
			// calculate the next batch
			startTime = endTime.Add(1 * time.Minute)
		}

		// we need to load this batch from the upstream
		cl, err := c.upstream(timeRange.ToInt64(startTime))
		if err != nil {
			log.Error().Err(err).Msg("could not create upstream")
			return
		}
		source, err := cl.Trades(make(chan api.Action), query)
		if err != nil {
			log.Error().Err(err).Msg("could not get trades from upstream")
			return
		}

		hash := c.hash.Do(startTime)
		k := c.key(hash, query.Coin)

		trades := make([]model.Trade, 0)
		var from *time.Time
		var to *time.Time
		for trade := range source {
			if from == nil {
				from = &trade.Time
			}
			to = &trade.Time
			// get the trades hash to see if it still belongs to our key
			h := c.hash.Do(trade.Time)
			// give an id to the trade
			if trade.ID == "" {
				trade.ID = uuid.New().String()
			}
			trades = append(trades, *trade)
			if h > hash {
				// signals to flush and start a new batch
				err = store.Store(k, trades)
				if err != nil {
					log.Error().Err(err).Msg("could not store trades")
					// dont exit here ... lets at least continue (?)
					// return
				}
				log.Info().
					Str("engine-uuid", query.Index).
					Time("from", *from).
					Time("to", *to).
					Err(err).
					Int64("hash", h).
					Msg("storing trade batch")
				hash = h
				k = c.key(hash, query.Coin)
				trades = make([]model.Trade, 0)
				from = nil
				//to = nil
			}
			trade.SourceID = query.Index
			c.trades[query.Coin] <- trade
			// wait for trade to be processed through the pipeline
			<-process
		}
		close(c.trades[query.Coin])
	}(c.timeRange, query)

	return c.trades[query.Coin], nil

}

func (c *Client) localTrades(uuid string, startTime time.Time, coin model.Coin, store storage.Persistence, process <-chan api.Action) (time.Time, error) {
	hash := c.hash.Do(startTime)
	k := c.key(hash, coin)
	trades := make([]model.Trade, 0)
	err := store.Load(k, &trades)
	log.Info().Err(err).
		Str("engine-uuid", uuid).
		Int("trades", len(trades)).
		Int64("hash", k.Hash).
		Time("from", startTime).
		Str("coin", string(coin)).
		Msg("loaded trades from local storage")
	if err != nil {
		return startTime, err
	}
	// just get all the local trades we got ... while updating our since index
	for _, localTrade := range trades {
		// add the meta map, which is ignored in the json
		if c.mock {
			localTrade.Live = true
			localTrade.Active = true
		} else {
			localTrade.Live = false
			localTrade.Active = false
		}
		localTrade.SourceID = uuid
		c.trades[coin] <- &localTrade
		startTime = localTrade.Time
		// wait for trade to be processed all the way through the pipeline
		<-process
	}
	return startTime, nil
}

func (c *Client) key(h int64, coin model.Coin) storage.Key {
	return storage.Key{
		Hash: h,
		Pair: string(coin),
		// TODO : make a method for this
		Label: fmt.Sprintf("from_%s_to_%s", c.hash.Undo(h).Format(timeFormat), c.hash.Undo(h+1).Format(timeFormat)),
	}
}
