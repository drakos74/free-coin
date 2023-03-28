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
	Live    bool   `json:"live"`
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
	HasUpdate bool                      `json:"-"`
	Trend     map[time.Duration]Trend   `json:"-"`
	Profit    map[time.Duration]*Profit `json:"-"`
	PnL       float64                   `json:"pnl"`
	Value     float64                   `json:"value"`
	Key       Key                       `json:"key"`
}

// Trend defines the position profit trend
type Trend struct {
	State        PositionState
	Live         bool
	Stamp        time.Time
	LastValue    []float64
	CurrentValue []float64
	Threshold    []float64
	CurrentDiff  []float64
	Type         []Type
	Shift        []Type
	XX           []float64
	YY           []float64
}

func (t Trend) Assess() ([]Type, bool) {
	validTrend := make([]Type, 2)
	hasTrend := false
	if t.Type[0] != NoType {
		// valid-trend
		validTrend[0] = t.Type[0]
		hasTrend = t.Live
	}
	if t.Type[1] != NoType {
		// valid-trend
		validTrend[1] = t.Type[1]
		hasTrend = t.Live
	}
	return validTrend, hasTrend
}

type PositionState struct {
	Coin         Coin
	OpenPrice    float64
	CurrentPrice float64
	PnL          float64
	Value        float64
	Type         Type
}

func newTrend(state PositionState) Trend {
	return Trend{
		State:        state,
		LastValue:    make([]float64, 2),
		CurrentValue: make([]float64, 2),
		Threshold:    make([]float64, 2),
		CurrentDiff:  make([]float64, 2),
		Type:         make([]Type, 2),
		Shift:        make([]Type, 2),
	}
}

// TrackingConfig defines the configuration for tracking position profit
type TrackingConfig struct {
	Duration  time.Duration
	Samples   int
	Threshold []float64
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
	if len(config.Threshold) < 2 {
		config.Threshold = []float64{0.0, 0.0}
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
	Decision     *Decision `json:"decision"`
	Coin         Coin      `json:"coin"`
	Type         Type      `json:"type"`
	OpenPrice    float64   `json:"open_price"`
	CurrentPrice float64   `json:"current_price"`
	Volume       float64   `json:"volume"`
}

func (p *Position) Sync(pos Position) (Position, bool) {
	update := false

	if pos.OpenPrice != p.OpenPrice {
		update = true
		p.OpenPrice = pos.OpenPrice
	}
	if pos.Volume != p.Volume {
		update = true
		p.Volume = pos.Volume
	}
	return *p, update
}

// Update updates the current status of the position and the profit or loss percentage.
func (p *Position) Update(trace bool, trade Tick, cfg []*TrackingConfig) Position {

	if trade.Active {
		p.CurrentPrice = trade.Range.To.Price
		p.CurrentTime = trade.Range.To.Time
	}

	pnl, value, fees := PnL(p.Type, p.Volume, p.OpenPrice, p.CurrentPrice)
	p.PnL = pnl
	p.Value = value
	p.Fees = fees

	p.HasUpdate = false

	if p.Profit == nil {
		profit := make(map[time.Duration]*Profit)
		configs := make([]string, 0)
		if cfg != nil && len(cfg) > 0 {
			// if we are given a tracking config , apply the history to the order
			for _, cfg := range cfg {
				if _, ok := profit[cfg.Duration]; ok {
					log.Warn().Str("key", fmt.Sprintf("%.0f", cfg.Duration.Minutes())).Msg("config already present")
				}
				configs = append(configs, fmt.Sprintf("%+v", cfg))
				profit[cfg.Duration] = NewProfit(cfg)
			}
		}
		p.Profit = profit
		log.Info().
			Str("coin", string(p.Coin)).
			Str("config", fmt.Sprintf("%v", len(profit))).
			Strs("configs", configs).
			Msg("create tracking config for position")
	}
	if p.Trend == nil {
		p.Trend = make(map[time.Duration]Trend)
	}

	if p.Profit != nil {
		state := PositionState{
			Coin:         p.Coin,
			Type:         p.Type,
			OpenPrice:    p.OpenPrice,
			CurrentPrice: trade.Price,
			Value:        p.Value,
			PnL:          p.PnL,
		}
		// try to ingest the new value to the window stats

		for k, profit := range p.Profit {
			if trend, ok := p.Trend[k]; !ok {
				p.Trend[k] = newTrend(state)
			} else {
				trend.Live = false
				trend.State = state
				p.Trend[k] = trend
			}

			//if trace {
			//	log.Info().
			//		Str("coin", string(p.Coin)).
			//		Str("duration", fmt.Sprintf("%+v", k.Minutes())).
			//		Str("config", fmt.Sprintf("%+v", profit.Config)).
			//		Str("time", fmt.Sprintf("%+v", trade.Time)).
			//		Str("index", fmt.Sprintf("%+v", trade.Time.Unix()/int64(profit.Config.Duration.Seconds()))).
			//		Str("pnl", fmt.Sprintf("%+v", p.PnL)).
			//		Str("window", fmt.Sprintf("%+v", profit.Window.Raw())).
			//		Msg("tracking")
			//}

			if _, ok := profit.Window.Push(trade.Time, p.PnL); ok {
				p.HasUpdate = true
				s, xx, yy, err := profit.Window.Polynomial(0, func(b buffer.TimeWindowView) float64 {
					return 100 * buffer.Avg(b)
				}, 1, trace)
				if err != nil {
					log.Warn().
						Str("coin", string(p.Coin)).Err(err).
						Floats64("xx", xx).
						Floats64("yy", yy).
						Msg("could not complete polynomial '2' fit for position")
				}
				a, xx, yy, err := profit.Window.Polynomial(0, func(b buffer.TimeWindowView) float64 {
					return 100 * buffer.Avg(b)
				}, 2, trace)
				if err != nil {
					log.Warn().Str("coin", string(p.Coin)).Err(err).
						Floats64("xx", xx).
						Floats64("yy", yy).
						Msg("could not complete polynomial '3' fit for position")
				}
				trend := p.Trend[k]
				trend.Stamp = time.Now()
				trend.Live = true
				trend.XX = xx
				trend.YY = yy
				if len(s) >= 1 && len(a) >= 2 {
					trend.CurrentValue[0] = s[1]
					trend.CurrentValue[1] = a[2]
					trend.CurrentDiff[0] = math.Abs(trend.CurrentValue[0]) - profit.Config.Threshold[0]
					trend.CurrentDiff[1] = math.Abs(trend.CurrentValue[1]) - profit.Config.Threshold[1]
					trend.Threshold = profit.Config.Threshold
					trend.Type[0], trend.Shift[0] = calculateTrend(trend.CurrentValue[0], profit.Config.Threshold[0], trend.LastValue[0])
					trend.Type[1], trend.Shift[1] = calculateTrend(trend.CurrentValue[1], profit.Config.Threshold[1], trend.LastValue[1])
					trend.LastValue[0] = s[1]
					trend.LastValue[1] = a[2]
					p.Trend[k] = trend
				}
			}
		}
	}
	return *p
}

func calculateTrend(currentValue, threshold, lastValue float64) (trend Type, shift Type) {
	if math.Abs(currentValue) > threshold {
		// NOTE : we identify with type , the loss conditions for this position, not the market movement
		trend = SignedType(currentValue)
		if currentValue*lastValue < 0 {
			//  we have a switch of direction
			shift = trend
		}
	}
	return trend, shift
}

type TrendReport struct {
	Profit           float64
	StopLossActive   bool
	TakeProfitActive bool
	ValidTrend       []Type
	ValidShift       []Type
}

// AssessTrend assesses the current position trend
func AssessTrend(pp map[Key]Position, takeProfit, stopLoss float64) (map[Key]Position, []float64, map[Key]map[time.Duration]Trend, map[Key]TrendReport) {
	positions := make(map[Key]Position)
	allProfit := make([]float64, 0)
	allTrend := make(map[Key]map[time.Duration]Trend)
	reports := make(map[Key]TrendReport, 0)
	if len(pp) > 0 {
		for k, position := range pp {
			profit := position.PnL
			stopLossActivated := position.PnL <= -1*stopLoss
			takeProfitActivated := position.PnL >= takeProfit
			report := TrendReport{
				Profit:           profit,
				StopLossActive:   stopLossActivated,
				TakeProfitActive: takeProfitActivated,
			}

			//shift := position.Trend.Shift != model.NoType
			//validShift := position.Trend.Shift != position.t
			// TODO : NOT multi-position ready !!!
			// we assume it s only one position thats relevant
			for tt, trend := range position.Trend {
				// NOTE : t here does not mean market , but Profit/Loss
				if validTrend, ok := trend.Assess(); ok {
					if _, ok := allTrend[k]; !ok {
						allTrend[k] = make(map[time.Duration]Trend)
					}
					allTrend[k][tt] = trend
					report.ValidTrend = validTrend
					report.ValidShift = trend.Shift
				}
			}
			//if stopLossActivated {
			//	// if we pass the stop-Loss threshold
			//	positions[k] = position
			//	delete(xt.Profit, k)
			//} else
			//if shift && validShift {
			//	// if there is a shift in the opposite direction of the position
			//	positions[k] = position
			//	delete(xt.Profit, k)
			//} else
			//fmt.Printf("[ valid = %v  , take-Profit = %v, stop-Loss = %v : %+v ]\n", validTrend, takeProfitActivated, stopLossActivated, Profit)
			if stopLossActivated || takeProfitActivated {
				// both signals for the trend need to be negative to produce a close signal
				if (len(report.ValidTrend) > 0 && report.ValidTrend[0] == Sell) &&
					(len(report.ValidTrend) > 1 && report.ValidTrend[1] == Sell) {
					positions[k] = position
				} else if (len(report.ValidTrend) > 0 && report.ValidShift[0] == Sell) ||
					(len(report.ValidTrend) > 1 && report.ValidShift[1] == Sell) {
					positions[k] = position
				}
			}
			reports[k] = report
			allProfit = append(allProfit, profit)
		}
	}
	return positions, allProfit, allTrend, reports
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
			Trend:  make(map[time.Duration]Trend),
			Profit: profit,
			Key:    order.Key,
		},
		Coin:      order.Coin,
		Type:      order.Type,
		Volume:    order.Volume,
		OpenPrice: order.Price,
	}
}
