package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/drakos74/free-coin/internal/model"
)

type Request struct {
	Coin model.Coin
	From time.Time
	To   time.Time
}

func NewRequest(r *RawRequest) (*Request, error) {
	coin := model.Coin(r.Coin[0])

	from, err := time.Parse("2006_01_02T15", r.From[0])
	if err != nil {
		return nil, fmt.Errorf("could not parse 'from' time: %w", err)
	}

	to, err := time.Parse("2006_01_02T15", r.To[0])
	if err != nil {
		return nil, fmt.Errorf("could not parse 'to' time: %w", err)
	}

	return &Request{
		Coin: coin,
		From: from,
		To:   to,
	}, nil

}

// RawRequest defines a backtest request
type RawRequest struct {
	Coin      []string `json:"coin"`
	From      []string `json:"from"`
	To        []string `json:"to"`
	Interval  []string `json:"interval"`
	Processor []string `json:"processor"`
	Prev      []string `json:"prev"`
	Next      []string `json:"next"`
	Threshold []string `json:"threshold"`
}

func parseQuery(values url.Values) (*RawRequest, error) {
	bb, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("could not parseQuery values: %w", err)
	}

	req := new(RawRequest)
	err = json.Unmarshal(bb, req)
	if err != nil {
		return nil, fmt.Errorf("could not parseQuery request: %w", err)
	}

	return req, nil

}

// TrainRequest defines a backtest request
type TrainRequest struct {
	Coin       []string `json:"coin"`
	From       []string `json:"from"`
	To         []string `json:"to"`
	Precision  []string `json:"precision"`
	BufferSize []string `json:"buffer"`
	Size       []string `json:"size"`
	Features   []string `json:"features"`
	Model      []string `json:"model"`
}

func parseTrain(values url.Values) (*TrainRequest, error) {
	bb, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("could not parseQuery values: %w", err)
	}

	req := new(TrainRequest)
	err = json.Unmarshal(bb, req)
	if err != nil {
		return nil, fmt.Errorf("could not parseQuery request: %w", err)
	}

	return req, nil

}

// Config defines a backtest config
type Config struct {
	Prev     int `json:"prev"`
	Next     int `json:"next"`
	Interval int `json:"interval"`
}

// Response defines the response structure for the backtest execution
type Response struct {
	Details []Details          `json:"details"`
	Time    []time.Time        `json:"time"`
	Trades  []Point            `json:"trades"`
	Price   []Point            `json:"price"`
	Loss    []Point            `json:"loss"`
	Trigger map[string]Trigger `json:"trigger"`
}

type Details struct {
	Coin     string `json:"coin"`
	Duration int    `json:"duration"`
	Prev     int    `json:"prev"`
	Next     int    `json:"next"`
	Result   Result `json:"result"`
}

type Trigger struct {
	Buy  []Point `json:"buy"`
	Sell []Point `json:"sell"`
}

type Point struct {
	X time.Time `json:"x"`
	Y float64   `json:"y"`
}

type Result struct {
	Trades    int     `json:"trades"`
	Threshold int     `json:"threshold"`
	Value     float64 `json:"value"`
	Coins     int     `json:"coins"`
	CoinValue float64 `json:"coinValue"`
	Fees      float64 `json:"fees"`
	PnL       float64 `json:"pnl"`
}

func (r Result) portfolio() float64 {
	return r.Value + (float64(r.Coins) * r.CoinValue) - r.Fees
}
