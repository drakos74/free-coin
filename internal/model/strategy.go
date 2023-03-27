package model

// Decision defines a trade/order strategy details
type Decision struct {
	Confidence float64   `json:"confidence"`
	Features   []float64 `json:"features"`
	Importance []float64 `json:"importance"`
	Config     []float64 `json:"config"`
	Position   Close     `json:"close"`
}

type Close struct {
	TakeProfit float64 `json:"take-profit"`
	StopLoss   float64 `json:"stop-loss"`
}
