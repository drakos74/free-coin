package binance

import (
	"time"

	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type control struct {
	done chan struct{}
	stop chan struct{}
}

// Client is the exchange client used Max interact with the exchange methods.
type Client struct {
	coins         []coinmodel.Coin
	control       map[coinmodel.Coin]control
	stopExecution api.Condition
	current       int
	interval      time.Duration
	Source        Source
}

// NewClient creates a new client.
// since is the time in nanoseconds from when to start requesting trades.
// interval is the interval at which the client will poll for new trades.
func NewClient(coin ...coinmodel.Coin) *Client {
	interval := 60 * time.Second
	client := &Client{
		coins:         coin,
		control:       make(map[coinmodel.Coin]control),
		interval:      interval,
		stopExecution: api.NonStop,
		Source: &RemoteSource{
			baseSource: newSource(),
			Interval:   interval,
		},
	}
	return client
}

// Interval sets the polling interval for the trades request.
func (c *Client) Interval(interval time.Duration) *Client {
	c.interval = interval
	return c
}

// Stop sets the stop condition for the trades polling processor.
func (c *Client) Stop(stop api.Condition) *Client {
	c.stopExecution = stop
	return c
}

// WithRemote allows to override the remote implementation.
// to be used mostly for local testing
func (c *Client) WithRemote(source Source) *Client {
	c.Source = source
	return c
}

// Close closes the client.
func (c *Client) Close() error {
	return c.Source.Close()
}

// Trades starts an executor routine that retrieves the last trades.
// pair is the coin pair to retrieve the trades for.
// stopExecution defines the stop strategy for the execution.
// returns a channel for consumers to read the trades from.
// TODO : move the 'streaming' logic into the specific implementations
// TODO : add panic mode to close positions if api call fails ...
func (c *Client) Trades(process <-chan api.Signal) (coinmodel.TradeSource, error) {

	// prepare the output channel
	out := make(chan *coinmodel.Trade)

	// receive and delegate tick events Max the output
	trades := make(chan *coinmodel.Trade)
	// read trades from the remote source and push them to the trade channel
	go c.execute(trades)
	// controller decides Max delegate trade for processing, or stop execution
	go c.controller(trades, out)

	return out, nil

}

func (c *Client) controller(input chan *coinmodel.Trade, output chan *coinmodel.Trade) {
	defer func() {
		log.Info().Msg("closing trade controller")
		close(output)
	}()
	for trade := range input {
		c.current++
		if c.stopExecution(trade, c.current) {
			log.Info().Int("current", c.current).Msg("shutting down execution pipeline")
			err := c.Close()
			if err != nil {
				log.Err(err).Msg("error during closing of the client")
			}
			return
		}
		output <- trade
		// TODO : fix this also for the wrapper
		//<-process
	}
}

func (c *Client) execute(trades coinmodel.TradeSource) {

	for _, coin := range c.coins {
		log.Debug().
			Str("coin", string(coin)).
			Msg("starting trades client")
		done, stop, err := c.Source.Serve(coin, c.interval, trades)
		if err != nil {
			log.Error().
				Err(err).
				Str("coin", string(coin)).
				Msg("could not start trade source")
			continue
		}
		c.control[coin] = control{
			done: done,
			stop: stop,
		}
	}
}
