package model

import (
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/buffer"
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

// Data contains position data and relevant keys
type Data struct {
	ID      string `json:"id"`
	TxID    string `json:"txId"`
	OrderID string `json:"order_id"`
	CID     string `json:"cid"`
}

// MetaData contains position related metadata
type MetaData struct {
	OpenTime    time.Time `json:"open_time"`
	CurrentTime time.Time `json:"current_time"`
	Fees        float64   `json:"fees"`
	Cost        float64   `json:"cost"`
	Net         float64   `json:"net"`
}

// Stats contains position stats, these are calculated by processors on the fly
type Stats struct {
	Trend  Trend                     `json:"-"`
	Profit map[time.Duration]*Profit `json:"-"`
	PnL    float64                   `json:"pnl"`
}

// Trend defines the position profit trend
type Trend struct {
	LastValue    float64
	CurrentValue float64
	Type         Type
	Shift        Type
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
}

// NewProfit creates a new position tracking struct
func NewProfit(config *TrackingConfig) *Profit {
	if config == nil {
		return nil
	}
	return &Profit{
		Config: *config,
		Window: buffer.NewHistoryWindow(config.Duration, config.Samples),
	}
}

// Position defines an open position details.
type Position struct {
	Data
	MetaData
	Stats
	Coin         Coin    `json:"coin"`
	Type         Type    `json:"type"`
	OpenPrice    float64 `json:"open_price"`
	CurrentPrice float64 `json:"current_price"`
	Volume       float64 `json:"volume"`
}

const timeDuration = time.Minute

// Update updates the current status of the position and the profit or loss percentage.
func (p *Position) Update(trade *Trade) Position {

	if trade != nil {
		p.CurrentPrice = trade.Price
		p.CurrentTime = trade.Time
	}

	net := 0.0
	switch p.Type {
	case Buy:
		net = p.CurrentPrice - p.OpenPrice
	case Sell:
		net = p.OpenPrice - p.CurrentPrice
	}

	if p.Fees == 0 {
		p.Fees = p.OpenPrice * p.Volume * Fees / 100
	}

	value := (net * p.Volume) - p.Fees
	profit := value / (p.OpenPrice * p.Volume)

	p.PnL = profit

	if p.Profit == nil || len(p.Profit) == 0 {
		p.Profit = map[time.Duration]*Profit{
			timeDuration: NewProfit(&TrackingConfig{
				Duration: timeDuration,
				Samples:  5,
			}),
		}
	}

	if p.Profit != nil {
		// try to ingest the new value to the window stats
		aa := make(map[time.Duration][]float64)
		vv := make(map[time.Duration][]float64)
		for k, _ := range p.Profit {
			if _, ok := p.Profit[k].Window.Push(trade.Time, profit); ok {
				a, err := p.Profit[k].Window.Polynomial(0, buffer.Avg, 2)
				if err != nil {
					log.Debug().Str("coin", string(p.Coin)).Err(err).Msg("could not complete polynomial fit for position")
				} else {
					aa[k] = a
				}
				v, err := p.Profit[k].Window.Values(0, buffer.Avg)
				if err != nil {
					log.Debug().Str("coin", string(p.Coin)).Err(err).Msg("could not extract values")
				} else {
					vv[k] = v
				}
			}
		}
		if a, ok := aa[timeDuration]; ok {
			if len(a) > 2 {
				p.Trend.CurrentValue = a[2]
				p.Trend.Type = NoType
				p.Trend.Shift = NoType
				if math.Abs(p.Trend.CurrentValue) > 0.0001 {
					p.Trend.Shift = SignedType(p.Trend.CurrentValue)
					if p.Trend.CurrentValue*p.Trend.LastValue < 0 {
						//  we have a switch of direction
						p.Trend.Type = SignedType(p.Trend.CurrentValue)
					}
				}
				p.Trend.LastValue = a[2]
			}
		}
	}
	return *p
}

// Value returns the value of the position and the profit or loss percentage.
// TODO : add processing function for past profits
func (p *Position) Value(price *Price) (value, profit float64, stats map[time.Duration][]float64) {

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

	if price == nil || p.Profit == nil {
		return value, profit, nil
	}

	// try to ingest the new value to the window stats
	aa := make(map[time.Duration][]float64)
	for k, _ := range p.Profit {
		if _, ok := p.Profit[k].Window.Push(price.Time, profit); ok {
			a, err := p.Profit[k].Window.Polynomial(0, func(b buffer.TimeWindowView) float64 {
				return b.Value
			}, 2)
			if err != nil {
				log.Debug().Str("coin", string(p.Coin)).Err(err).Msg("could not complete polynomial fit for position")
			} else {
				aa[k] = a
			}
		}
	}

	return value, profit, aa
}

// OpenPosition creates a position from a given order.
func OpenPosition(order *TrackedOrder, trackingConfig []*TrackingConfig) Position {
	profit := make(map[time.Duration]*Profit)
	if trackingConfig != nil && len(trackingConfig) > 0 {
		// if we are given a tracking config , apply the history to the order
		for _, cfg := range trackingConfig {
			if _, ok := profit[cfg.Duration]; ok {
				log.Warn().Str("key", fmt.Sprintf("%.0f", cfg.Duration.Minutes())).Msg("config already present")
			}
			profit[cfg.Duration] = NewProfit(cfg)
		}
	}
	return Position{
		Data: Data{
			ID:      uuid.New().String(),
			OrderID: order.ID,
		},
		MetaData: MetaData{
			OpenTime: order.Time,
		},
		Stats: Stats{
			Profit: nil,
		},
		Coin:      order.Coin,
		Type:      order.Type,
		Volume:    order.Volume,
		OpenPrice: order.Price,
	}
}
