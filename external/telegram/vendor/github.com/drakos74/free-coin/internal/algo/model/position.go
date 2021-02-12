package model

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

func OpenPosition(coin Coin, t Type) Position {
	return Position{
		Coin: coin,
		Type: t,
	}
}
