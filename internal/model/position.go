package model

import (
	"time"

	"github.com/drakos74/free-coin/internal/buffer"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/google/uuid"
)

// TrackedPosition is a wrapper for the position adding a timestamp of the position related event.
type TrackedPosition struct {
	Open  time.Time `json:"open"`
	Close time.Time `json:"close"`
	Position
}

type TrackedPositions []TrackedPosition

// for sorting predictions
func (p TrackedPositions) Len() int           { return len(p) }
func (p TrackedPositions) Less(i, j int) bool { return p[i].Open.Before(p[j].Open) }
func (p TrackedPositions) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// PositionBatch is a batch of open positions.
type PositionBatch struct {
	Positions []Position
	Index     int64
}

// EmptyBatch creates an empty batch.
func EmptyBatch() *PositionBatch {
	return &PositionBatch{
		Positions: []Position{},
	}
}

// FullBatch creates an empty batch.
func FullBatch(positions ...Position) *PositionBatch {
	return &PositionBatch{
		Positions: positions,
	}
}

type Data struct {
	ID      string `json:"id"`
	TxID    string `json:"txId"`
	OrderID string `json:"order_id"`
	CID     string `json:"cid"`
}

type MetaData struct {
	OpenTime    time.Time `json:"open_time"`
	CurrentTime time.Time `json:"current_time"`
	Fees        float64   `json:"fees"`
	Cost        float64   `json:"cost"`
	Net         float64   `json:"net"`
}

// TrackingConfig defines the configuration for tracking position profit
type TrackingConfig struct {
	Duration time.Duration
	Samples  int
}

// Track creates a new tracking config.
func Track(duration time.Duration, samples int) *TrackingConfig {
	return &TrackingConfig{
		Duration: duration,
		Samples:  samples,
	}
}

// Profit defines the profit tracking
type Profit struct {
	Config TrackingConfig       `json:"config"`
	Window buffer.HistoryWindow `json:"-"`
	Data   [][]float64          `json:"data"`
}

// NewProfit creates a new position tracking struct
func NewProfit(config *TrackingConfig) *Profit {
	if config == nil {
		return nil
	}
	return &Profit{
		Config: *config,
		Window: buffer.NewHistoryWindow(config.Duration, config.Samples),
		Data:   make([][]float64, 0),
	}
}

// Position defines an open position details.
type Position struct {
	Data
	MetaData
	Coin         Coin    `json:"coin"`
	Type         Type    `json:"type"`
	OpenPrice    float64 `json:"open_price"`
	CurrentPrice float64 `json:"current_price"`
	Volume       float64 `json:"volume"`
	Profit       *Profit `json:"profit"`
}

// Value returns the value of the position and the profit or loss percentage.
// TODO : add processing function for past profits
func (p *Position) Value(price *Price) (value, profit float64) {

	if price != nil {
		p.CurrentPrice = price.Value
	}

	net := 0.0
	switch p.Type {
	case Buy:
		net = p.CurrentPrice - p.OpenPrice
	case Sell:
		net = p.OpenPrice - p.CurrentPrice
	}
	value = (net * p.Volume) - p.Fees
	profit = 100 * value / (p.OpenPrice * p.Volume)

	// add the profit to the window
	if price != nil && p.Profit != nil {
		if w, ok := p.Profit.Window.Push(price.Time, profit); ok {
			p.Profit.Data = append(p.Profit.Data, []float64{w.Bucket.Values().Stats()[0].Avg(), float64(cointime.ToMilli(w.Time))})
		}
	}

	return value, profit
}

// OpenPosition creates a position from a given order.
func OpenPosition(order *TrackedOrder, trackingConfig *TrackingConfig) Position {
	return Position{
		Data: Data{
			ID:      uuid.New().String(),
			OrderID: order.ID,
		},
		MetaData: MetaData{
			OpenTime: order.Time,
		},
		Coin:      order.Coin,
		Type:      order.Type,
		Volume:    order.Volume,
		OpenPrice: order.Price,
		Profit:    NewProfit(trackingConfig),
	}
}
