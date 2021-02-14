package coinapi

// Coin defines a custom coin type
type Coin string

const (
	// NoCoin is a undefined coin
	NoCoin Coin = ""
	// BTC represents bitcoin
	BTC Coin = "BTC"
	// ETH represents the ethereum token
	ETH Coin = "ETH"
	// EOS represents the eos
	EOS Coin = "EOS"
	// LINK represents link
	LINK Coin = "LINK"
	// WAVES represents the waves token
	WAVES Coin = "WAVES"
	// DOT represents the dot
	DOT Coin = "DOT"
	// XRP represents the xrp token
	XRP Coin = "XRP"
)

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
