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
