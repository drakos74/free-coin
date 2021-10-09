package kraken

import (
	"context"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// Client is the exchange client used To interact with the exchange methods.
type Client struct {
	coins         []coinmodel.Coin
	init          int64
	since         map[coinmodel.Coin]int64
	stopExecution api.Condition
	current       int
	interval      time.Duration
	Source        Source
}

// NewClient creates a new client.
// Since is the time in nanoseconds from when to start requesting trades.
// Interval is the interval at which the client will poll for new trades.
// For tests , debugging or mocking input for the client use the mock source
// i.e. client.WithRemote(NewMockSource("testdata/response-trades"))
func NewClient(coin ...coinmodel.Coin) *Client {
	interval := 60 * time.Second
	client := &Client{
		coins:         coin,
		since:         make(map[coinmodel.Coin]int64),
		interval:      interval,
		stopExecution: api.NonStop,
		Source: &RemoteSource{
			baseSource: newSource(),
			Interval:   interval,
			public:     krakenapi.New("KEY", "SECRET"),
		},
	}
	return client
}

// Since sets the starting point for consuming trades.
func (c *Client) Since(since int64) *Client {
	c.init = since
	return c
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
func (c *Client) Trades(process <-chan api.Signal) (coinmodel.TradeSource, error) {

	// this is our first run ... so lets make sure we pass in the right since parameter
	for _, coin := range c.coins {
		c.since[coin] = c.init
	}

	out := make(chan *coinmodel.Trade)

	// receive and delegate tick events To the output
	trades := make(chan *coinmodel.Trade)

	// controller decides To delegate trade for processing, or stop execution
	go c.controller(trades, out, process)

	go c.execute(0, trades)

	return out, nil

}

func (c *Client) controller(input chan *coinmodel.Trade, output chan *coinmodel.Trade, process <-chan api.Signal) {
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
		<-process
	}
}

func (c *Client) execute(i int, trades coinmodel.TradeSource) {
	if i >= len(c.coins) {
		i = 0
	}
	coin := c.coins[i]
	since := c.since[coin]
	log.Trace().
		Str("coin", string(coin)).
		Int64("since", since).
		Msg("calling trades API")
	tradeResponse, err := c.Source.Trades(coin, since)
	if err != nil || tradeResponse == nil {
		log.Error().
			Err(err).
			Str("coin", string(coin)).
			Int64("since", since).
			Bool("has-response", tradeResponse != nil).
			Msg("could not load trades")
		return
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
	//status := cointime.FromNano(tradeResponse.Index)
	// calculate percentage ...
	//progress := status.Sub(start).Minutes()
	//total := time.Since(start).Minutes()
	//percent := 100 * (1 - (total-progress)/total)
	//if math.Abs(percent) < 97 {
	//	// TODO : send a message instead and improve the tracker spamming
	//	log.Info().Time("start", start).Str("coin", string(c.coin)).Str("percent", coinmath.Format(percent)).Msg("progress")
	//}
	c.since[coin] = tradeResponse.Index

	// call itself after the processing finishes
	time.AfterFunc(c.interval, func() {
		c.execute(i+1, trades)
	})
}

func (c *Client) CurrentPrice(ctx context.Context) (map[coinmodel.Coin]coinmodel.CurrentPrice, error) {
	panic("implement me")
}
