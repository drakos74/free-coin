package api

// PositionBatch is a batch of open positions.
type PositionBatch struct {
	Positions []Position
	Index     int64
}

// Position defines an open position details.
type Position struct {
	ID           string
	Coin         Coin
	Type         Type
	OpenPrice    float64
	CurrentPrice float64
	Volume       float64
	Cost         float64
	Net          float64
	Fees         float64
}

// NewPosition creates a new position.
func NewPosition(trade Trade) Position {
	return Position{
		Coin:      trade.Coin,
		Type:      trade.Type,
		OpenPrice: trade.Price,
	}
}

// Value returns the value of the position and the profit or loss percentage.
func (p *Position) Value() (value, percent float64) {
	net := 0.0
	switch p.Type {
	case Buy:
		net = p.CurrentPrice - p.OpenPrice
	case Sell:
		net = p.OpenPrice - p.CurrentPrice
	}
	value = (net * p.Volume) - p.Fees
	return value, 100 * value / (p.OpenPrice * p.Volume)
}
