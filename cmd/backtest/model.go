package main

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Request defines a backtest request
type Request struct {
	Coin      []string `json:"coin"`
	From      []string `json:"from"`
	To        []string `json:"to"`
	Processor []string `json:"processor"`
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
