package client

import "time"

// Report defines the client stats
type Report struct {
	Buy        int       `json:"buy"`
	BuyAvg     float64   `json:"buy_avg"`
	BuyVolume  float64   `json:"buy_vol"`
	Sell       int       `json:"sell"`
	SellAvg    float64   `json:"sell_avg"`
	SellVolume float64   `json:"sell_vol"`
	Wallet     float64   `json:"wallet"`
	Profit     float64   `json:"profit"`
	Fees       float64   `json:"fees"`
	LastPrice  float64   `json:"last_price"`
	Stamp      time.Time `json:"stamp"`
}
