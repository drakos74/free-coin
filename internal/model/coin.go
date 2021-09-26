package model

import "strings"

const EURO = "€"

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
	// MINA defines the MINA protocol token
	MINA Coin = "MINA"
)

// Coins contains coin related configuration.
var Coins = map[string]Coin{
	"BTC": BTC,
	"ETH": ETH,
	//"EOS":   EOS,
	"LINK": LINK,
	//"WAVES": WAVES,
	"DOT": DOT,
	//"XRP": XRP,
	"MINA": MINA,
}

func KnownCoins() []string {
	cc := make([]string, len(Coins))
	i := 0
	for c := range Coins {
		cc[i] = c
		i++
	}
	return cc
}

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

func TypeFromString(t string) Type {
	t = strings.ToUpper(t)
	switch t {
	case "TO":
		fallthrough
	case "BUY":
		return Buy
	case "FROM":
		fallthrough
	case "SELL":
		return Sell
	}
	return NoType
}

// SignedType returns the type based on the given sign.
func SignedType(v float64) Type {
	if v == -0 || v < 0 {
		return Sell
	}
	if v >= 0 {
		return Buy
	}
	return NoType
}

// Sign returns the appropriate sign for the given type for mathematical operations.
func (t Type) Sign() float64 {
	switch t {
	case Buy:
		return 1.0
	case Sell:
		return -1.0
	}
	return 0.0
}

// Inv inverts the type action.
func (t Type) Inv() Type {
	switch t {
	case Buy:
		return Sell
	case Sell:
		return Buy
	}
	return NoType
}

// String returns a human readable form of the type.
func (t Type) String() string {
	switch t {
	case Buy:
		return "buy"
	case Sell:
		return "sell"
	}
	return "none"
}
