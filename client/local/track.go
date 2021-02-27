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
)

// ExchangeTracker is a local exchange implementation that just tracks positions virtually
// It is used for back-testing
type ExchangeTracker struct {
	trades          map[model.Coin]model.Trade
	allTrades       map[model.Coin][]model.Trade
	positions       map[string]TrackedPosition
	closedPositions []TrackedPosition
	count           map[model.Coin]int
	mutex           *sync.Mutex
	logger          *log.Logger
	action          chan<- api.Action
}

// NewExchange creates a new local exchange
func NewExchange(logFile string, action chan<- api.Action) *ExchangeTracker {
	var logger *log.Logger
	if logFile != "" {
		file, err := os.OpenFile("cmd/test/logs/tracker_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		logger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
	return &ExchangeTracker{
		trades:          make(map[model.Coin]model.Trade),
		allTrades:       make(map[model.Coin][]model.Trade),
		positions:       make(map[string]TrackedPosition),
		closedPositions: make([]TrackedPosition, 0),
		count:           make(map[model.Coin]int),
		mutex:           new(sync.Mutex),
		logger:          logger,
		action:          action,
	}
}

func (e *ExchangeTracker) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
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
		Index:     0,
	}, nil
}

func (e *ExchangeTracker) log(msg string) {
	if e.logger != nil {
		e.logger.Println(msg)
	}
}

func (e *ExchangeTracker) OpenPosition(position model.Position) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	var price float64
	var time time.Time
	if trade, ok := e.trades[position.Coin]; ok {
		price = trade.Price
		time = trade.Time
	}
	position.OpenPrice = price
	trackedPosition := TrackedPosition{
		Open:     time,
		Position: position,
	}
	if _, ok := e.positions[position.ID]; ok {
		fmt.Println(fmt.Sprintf("duplicate position found for  = %+v", position.ID))
	}
	e.positions[position.ID] = trackedPosition
	//fmt.Println(fmt.Sprintf("open position = %+v", trackedPosition))
	e.log(fmt.Sprintf("open position = %+v", trackedPosition))
	return nil
}

func (e *ExchangeTracker) OpenOrder(order model.Order) error {
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
	trackedPosition := TrackedPosition{
		Open:     time,
		Position: position,
	}
	if _, ok := e.positions[position.ID]; ok {
		fmt.Println(fmt.Sprintf("duplicate position found for  = %+v", position.ID))
	}
	e.positions[order.ID] = trackedPosition
	//fmt.Println(fmt.Sprintf("open order = %+v", position))
	e.log(fmt.Sprintf("open order = %+v", position))
	return nil
}

func (e *ExchangeTracker) ClosePosition(position model.Position) error {
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
		fmt.Println(fmt.Sprintf("position not found = %+v", position.ID))
	}
	e.closedPositions = append(e.closedPositions, e.positions[position.ID])
	//fmt.Println(fmt.Sprintf("close position = %+v", e.positions[position.ID]))
	e.log(fmt.Sprintf("close position %+v", e.positions[position.ID]))
	return nil
}

func (e *ExchangeTracker) Process(trade *model.Trade) {
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
	e.allTrades[trade.Coin] = append(e.allTrades[trade.Coin], tr)
	e.trades[trade.Coin] = tr
	e.count[trade.Coin]++
	// signal to the source we are done processing this one
	e.action <- api.Action{}
}

func (e *ExchangeTracker) Gather() {
	fmt.Println(fmt.Sprintf("processing fininshed = %+v", len(e.count)))
	for c, cc := range e.count {
		fmt.Println(fmt.Sprintf("trades %s = %+v", string(c), cc))
	}
}

// Trades returns the processed trades.
func (e *ExchangeTracker) Trades(coin model.Coin) []model.Trade {
	return e.allTrades[coin]
}

func (e *ExchangeTracker) Positions(coin model.Coin) (positions []TrackedPosition) {
	return e.closedPositions
}
