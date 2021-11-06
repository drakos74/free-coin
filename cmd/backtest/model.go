package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Request defines a backtest request
type Request struct {
	Coin      []string `json:"coin"`
	From      []string `json:"from"`
	To        []string `json:"to"`
	Interval  []string `json:"interval"`
	Processor []string `json:"processor"`
	Prev      []string `json:"prev"`
	Next      []string `json:"next"`
}

// Config defines a backtest config
type Config struct {
	Prev     int `json:"prev"`
	Next     int `json:"next"`
	Interval int `json:"interval"`
}

func parse(values url.Values) (*Request, error) {
	bb, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("could not parse values: %w", err)
	}

	req := new(Request)
	err = json.Unmarshal(bb, req)
	if err != nil {
		return nil, fmt.Errorf("could not parse request: %w", err)
	}

	return req, nil

}

// Response defines the response structure for the backtest execution
type Response struct {
	Details []Details   `json:"details"`
	Time    []time.Time `json:"time"`
	Trades  []Point     `json:"trades"`
	Price   []Point     `json:"price"`
	Trigger Trigger     `json:"trigger"`
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
