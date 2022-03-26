package local

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/drakos74/free-coin/client"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	zlog "github.com/rs/zerolog/log"
)

const fee = 0.24 / 100

// Exchange is a local exchange implementation that just tracks positions virtually
// It is used for back-testing
type Exchange struct {
	oneOfEvery int
	trades     map[model.Coin]model.TradeSignal
	orders     []model.TrackedOrder
	allTrades  map[model.Coin][]model.TradeSignal
	count      map[model.Coin]int
	mutex      *sync.Mutex
	logger     *log.Logger
	processed  chan<- api.Signal
	wallet     map[model.Coin]wallet
	fees       float64
}

type wallet struct {
	value  float64
	volume float64
}

const (
	Terminal string = ""
	VoidLog  string = "VOID"
)

type VoidLogger struct {
}

func (vl VoidLogger) Write(p []byte) (n int, err error) {
	// nothing
	return 0, nil
}

// NewExchange creates a new local exchange
func NewExchange(logFile string) *Exchange {
	var logger *log.Logger
	switch logFile {
	case VoidLog:
		logger = log.New(VoidLogger{}, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	case Terminal:
	//nothing else to do
	default:
		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		logger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
	return &Exchange{
		trades:    make(map[model.Coin]model.TradeSignal),
		orders:    make([]model.TrackedOrder, 0),
		allTrades: make(map[model.Coin][]model.TradeSignal),
		count:     make(map[model.Coin]int),
		mutex:     new(sync.Mutex),
		wallet:    make(map[model.Coin]wallet),
		logger:    logger,
	}
}

// OneOfEvery defines how many trades to skip for the stats to return at the end of processing.
func (e *Exchange) OneOfEvery(n int) *Exchange {
	e.oneOfEvery = n
	return e
}

// SignalProcessed defines the channel to signal a trade having reached the end of the pipeline.
func (e *Exchange) SignalProcessed(processed chan<- api.Signal) *Exchange {
	e.processed = processed
	return e
}

func (e *Exchange) CurrentPrice(ctx context.Context) (map[model.Coin]model.CurrentPrice, error) {
	return make(map[model.Coin]model.CurrentPrice), nil
}

func (e *Exchange) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	return &model.PositionBatch{}, nil
}

func (e *Exchange) log(msg string) {
	if e.logger != nil {
		e.logger.Println(msg)
	} else {
		fmt.Printf("[local-exchange] %s\n", msg)
	}
}

func (e *Exchange) OpenOrder(order *model.TrackedOrder) (*model.TrackedOrder, []string, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// we assume here it s always market order
	//var price float64
	//var time time.Time
	//// re-calibrate market factors if we track this way (i.e. backtest etc...)
	//if trade, ok := e.trades[order.Coin]; ok {
	//	order.Price = price
	//	order.Time = trade.Time
	//}
	// TODO : use the tracking config
	e.orders = append(e.orders, *order)
	e.log(fmt.Sprintf("submit order = %+v", order))
	value := order.Price * order.Volume

	if _, ok := e.wallet[order.Coin]; !ok {
		e.wallet[order.Coin] = wallet{}
	}

	w := e.wallet[order.Coin]

	w.value -= value * fee
	switch order.Type {
	case model.Buy:
		w.value -= value
		w.volume += order.Volume
	case model.Sell:
		w.value += value
		w.volume -= order.Volume
	}
	e.wallet[order.Coin] = w
	return order, []string{order.ID}, nil
}

func (e *Exchange) Process(trade *model.TradeSignal) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// TODO : check why and when we are getting nil trades.
	if trade == nil {
		return
	}
	if _, ok := e.allTrades[trade.Coin]; !ok {
		e.allTrades[trade.Coin] = make([]model.TradeSignal, 0)
	}
	tr := *trade
	e.trades[trade.Coin] = tr
	e.count[trade.Coin]++
	if e.oneOfEvery == 0 || e.count[trade.Coin]%e.oneOfEvery == 0 {
		e.allTrades[trade.Coin] = append(e.allTrades[trade.Coin], tr)
	}
	// signal to the source we are done processing this one
	//e.processed <- api.Signal{}
}

func (e *Exchange) Gather(print bool) map[model.Coin]client.Report {
	if print {
		zlog.Info().Msg("processing finished")
		for c, cc := range e.count {
			zlog.Info().Str("coin", string(c)).Int("count", cc).Msg("trades")
			zlog.Info().Str("coin", string(c)).Int("count", len(e.allTrades[c])).Msg("all trades")
			zlog.Info().Str("coin", string(c)).Int("count", len(e.orders)).Msg("all orders")
			zlog.Info().Str("coin", string(c)).Float64("value", e.wallet[c].value).Msg("value")
			zlog.Info().Str("coin", string(c)).Float64("volume", e.wallet[c].volume).Msg("volume")
			zlog.Info().Str("coin", string(c)).Float64("price", e.trades[c].Tick.Price).Msg("price")
			zlog.Info().Str("coin", string(c)).Float64("pnl", e.wallet[c].value+e.trades[c].Tick.Price*e.wallet[c].volume).Msg("pnl")
		}
	}

	ww := make(map[model.Coin]client.Report)
	for _, o := range e.orders {
		if _, ok := ww[o.Coin]; !ok {
			ww[o.Coin] = client.Report{}
		}
		r := ww[o.Coin]
		switch o.Type {
		case model.Buy:
			r.Wallet -= o.Price * o.Volume
			r.Buy++
			r.BuyAvg += o.Price
			r.BuyVolume += o.Volume
		case model.Sell:
			r.Wallet += o.Price * o.Volume
			r.Sell++
			r.SellAvg += o.Price
			r.SellVolume += o.Volume
		}
		r.Fees += o.Price * o.Volume * model.Fees / 100
		// get the last price for this coin
		if tr, ok := e.trades[o.Coin]; ok {
			r.LastPrice = tr.Tick.Price
		}
		ww[o.Coin] = r
	}

	// do another pass
	for c, w := range ww {
		if w.Buy == 0 {
			w.BuyAvg = 0
		} else {
			w.BuyAvg = w.BuyAvg / float64(w.Buy)
		}
		if w.Sell == 0 {
			w.SellAvg = 0
		} else {
			w.SellAvg = w.SellAvg / float64(w.Sell)
		}
		// do the break-even calc
		f := w.SellVolume - w.BuyVolume
		w.Profit = w.Wallet - f*w.LastPrice
		ww[c] = w
	}
	if print {
		zlog.Info().Str("value", fmt.Sprintf("%+v", ww)).Msg("Wallet")
	}
	return ww
}

// Trades returns the processed trades.
func (e *Exchange) Trades(coin model.Coin) []model.TradeSignal {
	return e.allTrades[coin]
}

func (e *Exchange) Orders() (orders []model.TrackedOrder) {
	return e.orders
}

func (e *Exchange) Balance(ctx context.Context, priceMap map[model.Coin]model.CurrentPrice) (map[model.Coin]model.Balance, error) {
	// TODO :
	return make(map[model.Coin]model.Balance), nil
}

func (e *Exchange) Pairs(ctx context.Context) map[string]api.Pair {
	pairs := make(map[string]api.Pair)
	return pairs
}

type Noop struct {
}

func (n Noop) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	return nil, fmt.Errorf("noop 'OpenPositions'")
}

func (n Noop) OpenOrder(order model.TrackedOrder) (model.TrackedOrder, []string, error) {
	return order, []string{}, fmt.Errorf("noop 'OpenOrder'")
}

func (n Noop) ClosePosition(position model.Position) error {
	return fmt.Errorf("noop 'ClosePosition'")
}

func (n Noop) CurrentPrice(ctx context.Context) (map[model.Coin]model.CurrentPrice, error) {
	return make(map[model.Coin]model.CurrentPrice), fmt.Errorf("noop 'CurrentPrice'")
}

func (n Noop) Balance(ctx context.Context, priceMap map[model.Coin]model.CurrentPrice) (map[model.Coin]model.Balance, error) {
	return make(map[model.Coin]model.Balance), fmt.Errorf("noop 'Balance'")
}

func (n Noop) Pairs(ctx context.Context) map[string]api.Pair {
	pairs := make(map[string]api.Pair)
	return pairs
}
