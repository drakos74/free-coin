package model

import (
	"sort"
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
	From Event `json:"from"`
	To   Event `json:"to"`
}

// Tick defines a tick in the price
type Tick struct {
	Level  `json:"level"`
	Move   Move      `json:"move"`
	Range  Range     `json:"range"`
	Type   Type      `json:"type"`
	Time   time.Time `json:"time"`
	Active bool      `json:"active"`
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
	Time     time.Time `json:"time"`
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
	Book   Book   `json:"stats"`
	Spread Spread `json:"spread"`
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
//	Type     Type      `json:"Type"`
//	Active   bool      `json:"active"`
//	Live     bool      `json:"live"`
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

// TradeMap defines a generic grouping of trades
type TradeMap struct {
	Trades map[string]TradeSignal
	From   time.Time
	To     time.Time
	Order  []string
}

// NewTradeMap creates a new trade map from the given trades.
func NewTradeMap(trades ...TradeSignal) *TradeMap {
	m := make(map[string]TradeSignal, len(trades))

	// Sort by age, keeping original order or equal elements.
	sort.SliceStable(trades, func(i, j int) bool {
		return trades[i].Meta.Time.Before(trades[j].Meta.Time)
	})
	var from time.Time
	var to time.Time
	order := make([]string, len(trades))
	for i, trade := range trades {
		order[i] = trade.Meta.ID
		m[trade.Meta.ID] = trade
		if i == 0 {
			from = trade.Meta.Time
		}
		to = trade.Meta.Time
	}
	return &TradeMap{
		Trades: m,
		From:   from,
		To:     to,
		Order:  order,
	}
}

func (t *TradeMap) Append(m *TradeMap) {
	var newOrder []string
	// find the first in order
	if t.From.Before(m.From) {
		t.To = m.To
		newOrder = append(t.Order, m.Order...)
	} else {
		t.From = m.From
		newOrder = append(m.Order, t.Order...)
	}
	for id, trade := range m.Trades {
		t.Trades[id] = trade
	}
	t.Order = newOrder
}
