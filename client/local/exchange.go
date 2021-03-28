package local

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	zlog "github.com/rs/zerolog/log"
)

// Exchange is a local exchange implementation that just tracks positions virtually
// It is used for back-testing
type Exchange struct {
	oneOfEvery      int
	trades          map[model.Coin]model.Trade
	positions       map[string]model.TrackedPosition
	allTrades       map[model.Coin][]model.Trade
	closedPositions map[model.Coin][]model.TrackedPosition
	count           map[model.Coin]int
	mutex           *sync.Mutex
	logger          *log.Logger
	processed       chan<- api.Action
}

// NewExchange creates a new local exchange
func NewExchange(logFile string) *Exchange {
	var logger *log.Logger
	if logFile != "" {
		file, err := os.OpenFile("cmd/test/logs/tracker_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		logger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
	return &Exchange{
		trades:          make(map[model.Coin]model.Trade),
		positions:       make(map[string]model.TrackedPosition),
		allTrades:       make(map[model.Coin][]model.Trade),
		closedPositions: make(map[model.Coin][]model.TrackedPosition),
		count:           make(map[model.Coin]int),
		mutex:           new(sync.Mutex),
		logger:          logger,
	}
}

// OneOfEvery defines how many trades to skip for the stats to return at the end of processing.
func (e *Exchange) OneOfEvery(n int) *Exchange {
	e.oneOfEvery = n
	return e
}

// SignalProcessed defines the channel to signal a trade having reached the end of the pipeline.
func (e *Exchange) SignalProcessed(processed chan<- api.Action) *Exchange {
	e.processed = processed
	return e
}

func (e *Exchange) CurrentPrice(ctx context.Context) (map[model.Coin]model.CurrentPrice, error) {
	return make(map[model.Coin]model.CurrentPrice), nil
}

func (e *Exchange) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	positions := make([]model.Position, len(e.positions))
	i := 0
	for _, pos := range e.positions {
		positions[i] = pos.Position
		i++
	}
	return &model.PositionBatch{
		Positions: positions,
	}, nil
}

func (e *Exchange) log(msg string) {
	if e.logger != nil {
		e.logger.Println(msg)
	}
}

func (e *Exchange) OpenOrder(order model.TrackedOrder) (model.TrackedOrder, []string, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// we assume here it s always market order
	var price float64
	var time time.Time
	if trade, ok := e.trades[order.Coin]; ok {
		price = trade.Price
		time = trade.Time
	}
	order.Price = price
	position := model.OpenPosition(order)
	position.OpenTime = order.Time
	trackedPosition := model.TrackedPosition{
		Open:     time,
		Position: position,
	}
	if _, ok := e.positions[position.ID]; ok {
		zlog.Error().Str("id", position.ID).Msg("duplicate position found")
	}
	e.positions[position.ID] = trackedPosition
	e.log(fmt.Sprintf("open order = %+v", position))
	return order, []string{order.ID}, nil
}

func (e *Exchange) ClosePosition(position model.Position) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	var price float64
	var time time.Time
	if trade, ok := e.trades[position.Coin]; ok {
		price = trade.Price
		time = trade.Time
	}
	position.CurrentPrice = price
	if pos, ok := e.positions[position.ID]; ok {
		pos.Close = time
		pos.Position.CurrentPrice = price
		e.positions[position.ID] = pos
	} else {
		return fmt.Errorf("position not found: %s", position.ID)
	}
	if _, ok := e.closedPositions[position.Coin]; !ok {
		e.closedPositions[position.Coin] = make([]model.TrackedPosition, 0)
	}
	e.closedPositions[position.Coin] = append(e.closedPositions[position.Coin], e.positions[position.ID])
	delete(e.positions, position.ID)
	e.log(fmt.Sprintf("close position %+v", e.positions[position.ID]))
	return nil
}

func (e *Exchange) Process(trade *model.Trade) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// TODO : check why and when we are getting nil trades.
	if trade == nil {
		return
	}
	if _, ok := e.allTrades[trade.Coin]; !ok {
		e.allTrades[trade.Coin] = make([]model.Trade, 0)
	}
	tr := *trade
	e.trades[trade.Coin] = tr
	e.count[trade.Coin]++
	if e.oneOfEvery == 0 || e.count[trade.Coin]%e.oneOfEvery == 0 {
		e.allTrades[trade.Coin] = append(e.allTrades[trade.Coin], tr)
	}
	// signal to the source we are done processing this one
	e.processed <- api.Action{}
}

func (e *Exchange) Gather() {
	zlog.Info().Msg("processing finished")
	for c, cc := range e.count {
		zlog.Info().Str("coin", string(c)).Int("count", cc).Msg("trades")
		zlog.Info().Str("coin", string(c)).Int("count", len(e.allTrades[c])).Msg("all trades")
		zlog.Info().Str("coin", string(c)).Int("count", len(e.closedPositions[c])).Msg("closed positions")
	}
}

// Trades returns the processed trades.
func (e *Exchange) Trades(coin model.Coin) []model.Trade {
	return e.allTrades[coin]
}

func (e *Exchange) Positions(coin model.Coin) (positions []model.TrackedPosition) {
	return e.closedPositions[coin]
}
