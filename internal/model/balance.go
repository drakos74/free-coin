package model

// Balance represents the balance of an asset in the portfolio.
type Balance struct {
	Coin   Coin    `json:"coin"`
	Volume float64 `json:"volume"`
	Price  float64 `json:"price"`
	Locked float64 `json:"locked"`
}

// Value returns the value of the balance.
func (b Balance) Value() float64 {
	return b.Volume * b.Price
}

// PnL calculates the pnl of a current position based on the open price and current price.
func PnL(t Type, volume, openPrice, currentPrice float64) (float64, float64, float64) {
	net := 0.0
	switch t {
	case Buy:
		net = currentPrice - openPrice
	case Sell:
		net = openPrice - currentPrice
	}

	fees := openPrice * volume * Fees / 100

	value := (net * volume) - fees
	profit := value / (openPrice * volume)
	return profit, value, fees
}
