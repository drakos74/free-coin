package model

// Type defines the type of the order/movement buy or sell.
type Type byte

const (
	// NoType defines a missing trade type.
	NoType Type = iota
	// Buy defines a buy order.
	Buy
	// Sell defines a sell order.
	Sell
)

// String returns the string representation for the Type.
func (t Type) String() string {
	switch t {
	case Buy:
		return "buy"
	case Sell:
		return "sell"
	default:
		return ""
	}
}

// Sign returns the appropriate sign for the given Type.
func (t Type) Sign() float64 {
	switch t {
	case Buy:
		return 1.0
	case Sell:
		return -1.0
	}
	return 0.0
}

// Inv inverts the Type from buy to sell and vice versa.
func (t Type) Inv() Type {
	switch t {
	case Buy:
		return Sell
	case Sell:
		return Buy
	}
	return NoType
}
