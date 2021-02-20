package kraken

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/client/kraken/model"
	"github.com/drakos74/free-coin/internal/api"
	coinmath "github.com/drakos74/free-coin/internal/math"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

const (
	key    = "KRAKEN_KEY"
	secret = "KRAKEN_SECRET"
)

// Client is the exchange client used To interact with the exchange methods.
type Client struct {
	since    int64
	current  int
	interval time.Duration
	Api      *Remote
}

// New creates a new client.
// since is the time in nanoseconds from when to start requesting trades.
// interval is the interval at which the client will poll for new trades.
func New(ctx context.Context, since int64, interval time.Duration) *Client {
	client := &Client{
		since:    since,
		interval: interval,
		Api: &Remote{
			Interval:  interval,
			converter: model.NewConverter(),
			public:    krakenapi.New("KEY", "SECRET"),
			private:   krakenapi.New(os.Getenv(key), os.Getenv(secret)),
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
func (c *Client) Trades(stop <-chan struct{}, coin coinmodel.Coin, stopExecution api.Condition) (coinmodel.TradeSource, error) {

	out := make(chan *coinmodel.Trade)

	// receive and delegate tick events To the output
	trades := make(chan *coinmodel.Trade)

	// controller decides To delegate trade for processing, or stop execution
	go func(trades chan *coinmodel.Trade) {
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
	go cointime.Execute(stop, c.interval, func(trades chan<- *coinmodel.Trade) func() error {

		type state struct {
			last int64
		}

		s := &state{
			last: c.since,
		}

		start := cointime.FromNano(c.since)

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
				trades <- &trade //public.OpenTrade(coin, trade, active)
			}
			status := cointime.FromNano(tradeResponse.Index)
			// calculate percentage ...
			progress := status.Sub(start).Minutes()
			total := time.Since(start).Minutes()
			percent := 100 * (1 - (total-progress)/total)
			if math.Abs(percent) < 97 {
				// TODO : send a message instead
				log.Info().Str("coin", string(coin)).Str("percent", coinmath.Format(percent)).Msg("progress")
			}
			s.last = tradeResponse.Index
			return nil
		}
	}(trades),
		func() {
			log.Info().Str("pair", string(coin)).Msg("closing trade source")
			close(trades)
		})
	return out, nil

}

func (c *Client) ClosePosition(position coinmodel.Position) error {
	order := coinmodel.NewOrder(position.Coin).
		Market().
		WithVolume(position.Volume).
		WithLeverage(coinmodel.L_5).
		WithType(position.Type.Inv()).
		Create()

	_, _, err := c.Api.Order(order)
	if err != nil {
		return fmt.Errorf("could not close position: %w", err)
	}
	return nil
}

func (c *Client) OpenPosition(position coinmodel.Position) error {
	order := coinmodel.NewOrder(position.Coin).
		Market().
		WithVolume(position.Volume).
		WithLeverage(coinmodel.L_5).
		WithType(position.Type).
		Create()

	_, _, err := c.Api.Order(order)
	if err != nil {
		return fmt.Errorf("could not open position: %w", err)
	}
	return nil
}

func (c *Client) OpenPositions(ctx context.Context) (*coinmodel.PositionBatch, error) {
	params := map[string]string{
		"docalcs": "true",
	}
	response, err := c.Api.private.OpenPositions(params)
	if err != nil {
		return nil, fmt.Errorf("could not get positions: %w", err)
	}

	if response == nil {
		return nil, fmt.Errorf("received invalid response: %v", response)
	}

	positionsResponse := *response
	if len(positionsResponse) == 0 {
		return &coinmodel.PositionBatch{
			Positions: []coinmodel.Position{},
			Index:     time.Now().Unix(),
		}, nil
	}

	positions := make([]coinmodel.Position, len(positionsResponse))
	i := 0
	for k, pos := range *response {
		positions[i] = c.Api.newPosition(k, pos)
		i++
	}
	return &coinmodel.PositionBatch{
		Positions: positions,
		Index:     time.Now().Unix(),
	}, nil
}
