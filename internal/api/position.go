package api

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
