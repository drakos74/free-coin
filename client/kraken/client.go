package kraken

import (
	"fmt"
	"math"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/client/kraken/model"
	"github.com/drakos74/free-coin/internal/api"
	coinmath "github.com/drakos74/free-coin/internal/math"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

// Client is the exchange client used To interact with the exchange methods.
type Client struct {
	since         int64
	stopExecution api.Condition
	current       int
	interval      time.Duration
	Api           *RemoteClient
}

// NewClient creates a new client.
// since is the time in nanoseconds from when to start requesting trades.
// interval is the interval at which the client will poll for new trades.
func NewClient(since int64, interval time.Duration, stopExecution api.Condition) *Client {
	client := &Client{
		since:         since,
		interval:      interval,
		stopExecution: stopExecution,
		Api: &RemoteClient{
			Interval:  interval,
			converter: model.NewConverter(),
			public:    krakenapi.New("KEY", "SECRET"),
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
func (c *Client) Trades(process <-chan api.Action, query api.Query) (coinmodel.TradeSource, error) {

	out := make(chan *coinmodel.Trade)

	// receive and delegate tick events To the output
	trades := make(chan *coinmodel.Trade)

	// channel for stopping the execution
	stop := make(chan struct{})

	// controller decides To delegate trade for processing, or stop execution
	go func(trades chan *coinmodel.Trade) {
		defer func() {
			log.Info().Msg("closing trade controller")
			close(out)
			stop <- struct{}{}
		}()
		for trade := range trades {
			c.current++
			if c.stopExecution(trade, c.current) {
				log.Info().Int("current", c.current).Msg("shutting down execution pipeline")
				err := c.Close()
				if err != nil {
					log.Err(err).Msg("error during closing of the client")
				}
				return
			}
			out <- trade
			// TODO : fix this also for the wrapper
			//<-process
		}
	}(trades)

	// executor for polling trades
	go cointime.Execute(stop, c.interval, func(trades chan<- *coinmodel.Trade) func() error {

		type state struct {
			last int64
		}

		s := &state{
			last: c.since,
		}

		start := cointime.FromNano(c.since)

		return func() error {
			tradeResponse, err := c.Api.Trades(query.Coin, s.last)
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
				trades <- &trade //public.OpenTrade(coin, trade, active)
			}
			status := cointime.FromNano(tradeResponse.Index)
			// calculate percentage ...
			progress := status.Sub(start).Minutes()
			total := time.Since(start).Minutes()
			percent := 100 * (1 - (total-progress)/total)
			if math.Abs(percent) < 97 {
				// TODO : send a message instead and improve the tracker spamming
				log.Info().Time("start", start).Str("coin", string(query.Coin)).Str("percent", coinmath.Format(percent)).Msg("progress")
			}
			s.last = tradeResponse.Index
			return nil
		}
	}(trades),
		func() {
			log.Info().Str("pair", string(query.Coin)).Msg("closing trade source")
			close(trades)
		})
	return out, nil

}
