package kraken

import (
	"context"
	"fmt"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/coinapi"
	"github.com/drakos74/free-coin/external/kraken/api"
	cointime "github.com/drakos74/free-coin/time"
	"github.com/rs/zerolog/log"
)

// Client is the exchange client used To interact with the exchange methods.
type Client struct {
	since    int64
	current  int
	interval time.Duration
	Api      *api.Remote
}

// New creates a new client.
// since is the time in nanoseconds from when to start requesting trades.
// interval is the interval at which the client will poll for new trades.
func New(ctx context.Context, since int64, interval time.Duration) *Client {
	client := &Client{
		since:    since,
		interval: interval,
		Api: &api.Remote{
			PublicApi:  krakenapi.New("KEY", "SECRET"),
			PrivateApi: krakenapi.New(Key, Secret),
		},
	}
	return client
}

// Close closes the client.
func (c *Client) Close() error {
	return c.Api.Close()
}

// Trades starts an executor routine that retrieves the last trades.
// pair is the coin pair to retrieve the trades for.
// stopExecution defines the stop strategy for the execution.
// returns a channel for consumers to read the trades from.
// TODO : move the 'streaming' logic into the specific implementations
// TODO : add panic mode to close positions if api call fails ...
func (c *Client) Trades(ctx context.Context, coin coinapi.Coin, stopExecution coinapi.Condition) coinapi.TradeSource {

	out := make(chan coinapi.Trade)

	// receive and delegate tick events To the output
	trades := make(chan coinapi.Trade)

	// controller decides To delegate trade for processing, or stop execution
	go func(trades chan coinapi.Trade) {
		defer func() {
			log.Info().Msg("closing trade controller")
			close(out)
		}()
		for trade := range trades {
			c.current++
			if stopExecution(trade, c.current) {
				log.Info().Int("current", c.current).Msg("shutting down execution pipeline")
				err := c.Close()
				if err != nil {
					log.Err(err).Msg("error during closing of the client")
				}
				return
			}
			out <- trade
		}
	}(trades)

	// executor for polling trades
	go cointime.Execute(ctx, c.interval, func(trades chan<- coinapi.Trade) func() error {

		type state struct {
			last int64
		}

		s := &state{
			last: c.since,
		}

		return func() error {
			tradeResponse, err := c.Api.Trades(coin, s.last)
			if err != nil || tradeResponse == nil {
				return fmt.Errorf("could not get trades info: %w", err)
			}
			batchSize := len(tradeResponse.Trades)
			for i, trade := range tradeResponse.Trades {
				var active bool
				if i >= batchSize-1 {
					active = true
				}
				// signal the end of the trades batch
				trade.Active = active
				trades <- trade //public.OpenTrade(coin, trade, active)
			}
			s.last = tradeResponse.Index
			return nil
		}

	}(trades),
		func() {
			log.Info().Str("pair", string(coin)).Msg("closing trade source")
			close(trades)
		})

	return out

}

func (c *Client) ClosePosition(position coinapi.Position) error {
	return fmt.Errorf("not implemented ClosePosition for %+v", position)
}

func (c *Client) OpenPosition(position coinapi.Position) error {
	return fmt.Errorf("not implemented OpenPosition for %+v", position)
}