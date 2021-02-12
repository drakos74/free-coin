package model

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

// Config contains coin related configuration.
var Coins = map[string]Coin{
	"BTC":   BTC,
	"ETH":   ETH,
	"EOS":   EOS,
	"LINK":  LINK,
	"WAVES": WAVES,
	"DOT":   DOT,
	"XRP":   XRP,
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
