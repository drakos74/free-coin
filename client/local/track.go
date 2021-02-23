package local

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/drakos74/free-coin/internal/model"
)

type stats struct {
	profit float64
	loss   float64
	num    int
}

func newStats() *stats {
	return &stats{}
}

func (s *stats) add(profit float64) (total, avg float64, n int) {
	s.num++
	if profit > 0 {
		s.profit += profit
	} else {
		s.loss += profit
	}
	return s.result()
}

func (s *stats) result() (total, avg float64, n int) {
	total = s.profit + s.loss
	return total, total / float64(s.num), s.num
}

// ExchangeTracker is a local exchange implementation that just tracks positions virtually
// It is used for back-testing
type ExchangeTracker struct {
	trades    map[model.Coin]model.Trade
	positions map[string]model.Position
	count     map[model.Coin]int
	mutex     *sync.Mutex
	logger    *log.Logger
	pnl       map[model.Coin]*stats
}

// NewExchange creates a new local exchange
func NewExchange() *ExchangeTracker {
	file, err := os.OpenFile("cmd/test/logs/tracker_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	return &ExchangeTracker{
		trades:    make(map[model.Coin]model.Trade),
		positions: make(map[string]model.Position),
		count:     make(map[model.Coin]int),
		mutex:     new(sync.Mutex),
		logger:    log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		pnl:       make(map[model.Coin]*stats),
	}
}

func (e ExchangeTracker) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	positions := make([]model.Position, len(e.positions))
	i := 0
	for _, pos := range e.positions {
		positions[i] = pos
		i++
	}
	return &model.PositionBatch{
		Positions: positions,
		Index:     0,
	}, nil
}

func (e ExchangeTracker) OpenPosition(position model.Position) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.logger.Println(fmt.Sprintf("%v open position = %+v", e.trades[position.Coin].Time, position))
	e.positions[position.ID] = position
	return nil
}

func (e ExchangeTracker) OpenOrder(order model.Order) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// we assume here it s always market order
	order.Price = e.trades[order.Coin].Price
	position := model.OpenPosition(order)
	e.logger.Println(fmt.Sprintf("%v open order = %+v", e.trades[position.Coin].Time, position))
	e.positions[order.ID] = position
	return nil
}

func (e ExchangeTracker) ClosePosition(position model.Position) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	position.CurrentPrice = e.trades[position.Coin].Price
	e.logger.Println(fmt.Sprintf("%v close position %+v", e.trades[position.Coin].Time, position))
	// calculate Profit&Loss
	_, profit := position.Value()
	if _, ok := e.pnl[position.Coin]; !ok {
		e.pnl[position.Coin] = newStats()
	}
	total, avg, n := e.pnl[position.Coin].add(profit)
	fmt.Println(fmt.Sprintf("%s|%d profit = %v , avg = %v", position.Coin, n, total, avg))
	e.logger.Println(fmt.Sprintf("%s|%d profit = %v , avg = %v", position.Coin, n, total, avg))
	delete(e.positions, position.ID)
	return nil
}

func (e ExchangeTracker) Process(trade *model.Trade) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.trades[trade.Coin] = *trade
	// TODO : keep track of other properties of the trades
	e.count[trade.Coin]++
	if e.count[trade.Coin]%1000 == 0 {
		fmt.Println(fmt.Sprintf("%s = %d", trade.Coin, e.count[trade.Coin]))
	}
}

func (e ExchangeTracker) Gather() {
	panic("implement me")
}
