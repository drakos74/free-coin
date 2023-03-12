package kraken

import (
	"context"
	"strconv"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
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
	timer         map[coinmodel.Coin]time.Time
	socket        *Socket
	live          bool
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
		timer:  make(map[coinmodel.Coin]time.Time),
		socket: NewSocket(coin...),
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

// Live defines the trade pull strategy for the client
func (c *Client) Live(live bool) *Client {
	c.live = live
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
func (c *Client) Trades(process <-chan api.Signal) (out coinmodel.TradeSource, err error) {
	// this is our first run ... so lets make sure we pass in the right since parameter
	for _, coin := range c.coins {
		c.since[coin] = c.init
	}

	// expose the trades to the outside world
	out = make(chan *coinmodel.TradeSignal)

	// TODO : load past trades to train the model and then switch to socket ...
	if c.live {
		out, err = c.socket.Run(process)
	} else {
		// receive and delegate tick events To the output
		trades := make(chan *coinmodel.TradeSignal)

		// pull the next batch of trades
		go c.execute(0, trades)

		// controller decides To delegate trade for processing, or stop execution
		go c.controller(trades, out, process)
	}

	return out, err

}

const (
	controllerProcessor = "controller"
)

func (c *Client) controller(input coinmodel.TradeSource, output coinmodel.TradeSource, process <-chan api.Signal) {
	defer func() {
		log.Info().Msg("closing trade controller")
		close(output)
	}()
	for trade := range input {
		metrics.Observer.IncrementTrades(string(trade.Coin), controllerProcessor, "check")
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

const (
	sourceProcessor = "source"
	krakenProcessor = "kraken"
)

func (c *Client) execute(i int, trades coinmodel.TradeSource) {
	if i >= len(c.coins) {
		i = 0
	}

	// track how often we make the call for each coin
	now := time.Now()
	last := c.timer[c.coins[i]]
	duration := now.Sub(last).Seconds()
	f, _ := strconv.ParseFloat(now.Format("0102.1504"), 64)
	metrics.Observer.NoteLag(f, string(c.coins[i]), krakenProcessor, sourceProcessor)
	metrics.Observer.TrackDuration(duration, string(c.coins[i]), krakenProcessor, sourceProcessor)
	c.timer[c.coins[i]] = now
	// call itself after the processing finishes
	defer time.AfterFunc(c.interval, func() {
		interval := time.Now().Sub(now).Seconds()
		metrics.Observer.TrackDuration(interval, "ANY", krakenProcessor, sourceProcessor)
		c.execute(i+1, trades)
	})

	// do the execution logic for this cycle
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
	metrics.Observer.AddTrades(float64(batchSize), string(coin), krakenProcessor, sourceProcessor)
	for i, trade := range tradeResponse.Trades {
		var active bool
		if i >= batchSize-1 {
			active = true
		}
		// signal the end of the trades batch
		trade.Tick.Active = active
		trades <- &coinmodel.TradeSignal{
			Coin: trade.Coin,
			Meta: coinmodel.Meta{
				Time:     trade.Meta.Time,
				Size:     batchSize,
				Exchange: trade.Meta.Exchange,
			},
			Tick: trade.Tick,
		} //public.OpenTrade(coin, trade, active)
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
}

func (c *Client) CurrentPrice(ctx context.Context) (map[coinmodel.Coin]coinmodel.CurrentPrice, error) {
	panic("implement me")
}
