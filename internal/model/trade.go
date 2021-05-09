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
type TradeSource chan *Trade

// Trade represents a trade object with all necessary details.
type Trade struct {
	SourceID string    `json:"-"`
	ID       string    `json:"id"`
	RefID    string    `json:"refId"`
	Net      float64   `json:"net"`
	Coin     Coin      `json:"coin"`
	Price    float64   `json:"Value"`
	Volume   float64   `json:"Volume"`
	Time     time.Time `json:"time"`
	Type     Type      `json:"Type"`
	Active   bool      `json:"active"`
	Live     bool      `json:"live"`
	Signals  []Signal  `json:"-"`
}

// TradeBatch is an indexed group of trades
type TradeBatch struct {
	Trades []Trade
	Index  int64
}

// TradeMap defines a generic grouping of trades
type TradeMap struct {
	Trades map[string]Trade
	From   time.Time
	To     time.Time
	Order  []string
}

// NewTradeMap creates a new trade map from the given trades.
func NewTradeMap(trades ...Trade) *TradeMap {
	m := make(map[string]Trade, len(trades))

	// Sort by age, keeping original order or equal elements.
	sort.SliceStable(trades, func(i, j int) bool {
		return trades[i].Time.Before(trades[j].Time)
	})
	var from time.Time
	var to time.Time
	order := make([]string, len(trades))
	for i, trade := range trades {
		order[i] = trade.ID
		m[trade.ID] = trade
		if i == 0 {
			from = trade.Time
		}
		to = trade.Time
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
