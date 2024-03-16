package model

import (
	"time"
)

// Signal is a generic envelop for packing generic objects and passing them from process to process.
type Signal struct {
	Type  string
	Value interface{}
}

// VoidSignal is a void signal
var VoidSignal = Signal{}

// TradeSource is a channel for receiving and sending trades.
type TradeSource chan *TradeSignal

// SignalSource is a channel for receiving and sending trade signals.
type SignalSource chan *TradeSignal

// Level defines a trading level.
type Level struct {
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
}

// Depth defines a trading level.
type Depth struct {
	Count  float64 `json:"count"`
	Volume float64 `json:"volume"`
}

// Move describes the price move
type Move struct {
	Velocity float64 `json:"velocity"`
	Momentum float64 `json:"momentum"`
}

type Event struct {
	Price float64   `json:"price"`
	Time  time.Time `json:"time"`
}

type Range struct {
	Min Event `json:"min"`
	Max Event `json:"max"`
}

// Tick defines a tick in the price
type Tick struct {
	Level     `json:"level"`
	StatsData StatsData `json:"stats_data"`
	Move      Move      `json:"move"`
	Range     Range     `json:"range"`
	Type      Type      `json:"type"`
	Time      time.Time `json:"time"`
	Active    bool      `json:"active"`
}

type StatsData struct {
	Std   Level `json:"std"`
	Trend Level `json:"trend"`
	Buy   Depth `json:"buy"`
	Sell  Depth `json:"sell"`
}

func NewTick(price float64, volume float64, t Type, time time.Time) Tick {
	return Tick{
		Level: Level{
			Price:  price,
			Volume: volume,
		},
		Range: Range{
			Min: Event{
				Price: price,
				Time:  time,
			},
			Max: Event{
				Price: price,
				Time:  time,
			},
		},
		Type:   t,
		Time:   time,
		Active: true,
	}
}

// Spread defines the current spread
type Spread struct {
	Bid   Level
	Ask   Level
	Trade bool
	Time  time.Time
}

type Meta struct {
	ID       string    `json:"id"`
	Init     time.Time `json:"init"`
	First    time.Time `json:"first"`
	Time     time.Time `json:"time"`
	Unix     int64     `json:"unix"`
	Incr     int       `json:"incr"`
	Size     int       `json:"size"`
	Live     bool      `json:"live"`
	Exchange string    `json:"exchange"`
}

type Book struct {
	Count int
	Mean  float64
	Ratio float64
	Std   float64
}

// TradeSignal defines a generic trade signal
type TradeSignal struct {
	Coin   Coin   `json:"coin"`
	Meta   Meta   `json:"meta"`
	Tick   Tick   `json:"tick"`
	Book   Book   `json:"-"`
	Spread Spread `json:"-"`
}

//// Trade represents a trade object with all necessary details.
//type Trade struct {
//	SourceID string    `json:"-"`
//	Exchange string    `json:"exchange"`
//	ID       string    `json:"id"`
//	RefID    string    `json:"refId"`
//	Net      float64   `json:"net"`
//	Coin     Coin      `json:"coin"`
//	Price    float64   `json:"Value"`
//	Volume   float64   `json:"Volume"`
//	Time     time.Time `json:"time"`
//	t     t      `json:"t"`
//	Active   bool      `json:"active"`
//	Multi     bool      `json:"live"`
//	Meta     Batch     `json:"batch"`
//	Signals  []Signal  `json:"-"`
//}

type Batch struct {
	Duration time.Duration `json:"interval"`
	Price    float64       `json:"price"`
	Num      int           `json:"num"`
	Volume   float64       `json:"volume"`
}

// TradeBatch is an indexed group of trades
type TradeBatch struct {
	Trades []TradeSignal
	Index  int64
}
