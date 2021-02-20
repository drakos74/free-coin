package model

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/model"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/rs/zerolog/log"
)

const (
	XLINKZEUR  = "LINKEUR"
	XWAVESZEUR = "WAVESEUR"
	XDOTZEUR   = "DOTEUR"
	XXRPZEUR   = "XXRPZEUR"
)

// Coin creates a new coin converter for kraken.
func Coin() CoinConverter {
	return CoinConverter{coins: map[model.Coin]string{
		model.BTC:   krakenapi.XXBTZEUR,
		model.ETH:   krakenapi.XETHZEUR,
		model.EOS:   krakenapi.EOSEUR,
		model.LINK:  XLINKZEUR,
		model.WAVES: XWAVESZEUR,
		model.DOT:   XDOTZEUR,
		model.XRP:   XXRPZEUR,
	}}
}

// CoinConverter converts from the internal coin representation to kraken specific model
type CoinConverter struct {
	coins map[model.Coin]string
}

// Pair transforms the internal coin type to an exchange traded pair.
func (c CoinConverter) Pair(p model.Coin) string {
	fmt.Println(fmt.Sprintf("p = %+v", p))
	fmt.Println(fmt.Sprintf("c.coins = %+v", c.coins))
	if coin, ok := c.coins[p]; ok {
		return coin
	}
	panic(fmt.Sprintf("unknown coin %s", p))
}

// Coin transforms the kraken coin representation to the internal coin types.
func (c CoinConverter) Coin(p string) model.Coin {
	for coin, pair := range c.coins {
		if pair == p {
			return coin
		}
	}
	panic(fmt.Sprintf("unknown pair %s", p))
}

// Type creates a new type converter for kraken.
func Type() TypeConverter {
	return TypeConverter{types: map[model.Type]string{
		model.Buy:  "buy",
		model.Sell: "sell",
	}}
}

// TypeConverter converts between kraken and internal model types.
type TypeConverter struct {
	types map[model.Type]string
}

// To transforms from a kraken type representation to the internal coin model.
func (t TypeConverter) To(s string) model.Type {
	for t, ts := range t.types {
		if ts == s {
			return t
		}
	}
	log.Error().Str("type", s).Msg("unexpected type")
	return model.NoType
}

// From transforms from the internal model representation to the kraken model.
func (t TypeConverter) From(s model.Type) string {
	if ts, ok := t.types[s]; ok {
		return ts
	}
	log.Error().Str("type", string(s)).Msg("unexpected type")
	return ""
}

var NullResponse = &krakenapi.TradesResponse{
	Trades: []krakenapi.TradeInfo{
		{},
	},
}
