package local

import (
	"context"
	"errors"
	"fmt"
	"time"

	cointime "github.com/drakos74/free-coin/internal/time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
)

// Client will retrieve trades from the given key value storage.
// It will be able to retrieve and store data from an upstream client,
// if the storage does not contain the requested data.
// furthermore it will mock the client behaviour in terms of the positions and orders locally.
// It can be used as a simulation environment for testing processors and business logic.
type Client struct {
	since       int64
	upstream    func(since int64) (api.TradeClient, error)
	trades      model.TradeSource
	persistence storage.Persistence
}

// NewClient creates a new client for trade processing.
func NewClient(ctx context.Context, since int64) *Client {
	return &Client{
		since:  since,
		trades: make(chan *model.Trade),
	}
}

// WithUpstream adds an upstream client to the local client.
func (c *Client) WithUpstream(upstream func(since int64) (api.TradeClient, error)) *Client {
	c.upstream = upstream
	return c
}

// WithPersistence adds a storage layer to the local client.
func (c *Client) WithPersistence(persistence storage.Persistence) *Client {
	c.persistence = persistence
	return c
}

// TODO : make these in the time package
// hash hashes the given trade to the appropriate bucket
func (c *Client) hash(t time.Time) int64 {
	return t.Unix() / int64(time.Hour.Seconds())
}

// rehash retrieves the original data from the given hash
func (c *Client) rehash(s int64) time.Time {
	return time.Unix(s, 0)
}

// Trades returns the trades starting at since.
// it will try to get trades from the persistence layer if available by day
// otherwise it will call the upstream client if available.
// In that sense it works always in batches of one day interval.
func (c Client) Trades(stop <-chan struct{}, coin model.Coin, stopExecution api.Condition) (model.TradeSource, error) {

	// check if we have trades in the store ...
	start := cointime.FromNano(c.since)

	h := c.hash(start)

	k := storage.Key{
		Since: start.Unix(),
		Pair:  string(coin),
		Label: fmt.Sprintf("%d_%d", h, h+1),
	}

	trades := make([]model.Trade, 0)

	err := c.persistence.Load(k, trades)

	if errors.Is(err, storage.NotFoundErr) {
		// we need to load this batch from the upstream
		cl, err := c.upstream(start.UnixNano())
		if err != nil {
			return nil, fmt.Errorf("could not create upstream: %w", err)
		}
		stop := make(chan struct{})
		source, err := cl.Trades(stop, coin, api.NonStop)
		if err != nil {
			return nil, fmt.Errorf("could not get trades from upstream: %w", err)
		}
		for trade := range source {
			// get the trades hash to see if it still belongs to our key
			hash := c.hash(trade.Time)
			if hash == h {
				trades = append(trades, *trade)
			} else if hash > h {
				// the case where we got the first trade that is bigger than what we intendded to get
				stop <- struct{}{}
			}
		}
		// we should have stopped now, because the channel should have closed.
		err = c.persistence.Store(k, trades)
		if err != nil {
			return nil, fmt.Errorf("could not store trades batch: %w", err)
		}
	}

	return c.trades, nil

}

func (c Client) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	panic("implement me")
}

func (c Client) OpenPosition(position model.Position) error {
	panic("implement me")
}

func (c Client) ClosePosition(position model.Position) error {
	panic("implement me")
}
