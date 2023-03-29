package model

// Decision defines a trade/order strategy details
type Decision struct {
	Confidence float64   `json:"confidence"`
	Features   []float64 `json:"features"`
	Importance []float64 `json:"importance"`
	Config     []float64 `json:"config"`
	Boundary   Boundary  `json:"boundary"`
}

type Boundary struct {
	TakeProfit float64 `json:"take-profit"`
	StopLoss   float64 `json:"stop-loss"`
	Score      float64 `json:"score"`
	Limit      float64 `json:"limit"`
}
