package model

import (
	"github.com/adshao/go-binance/v2"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// Coin creates a new coin converter for binance.
func Coin() CoinConverter {
	return CoinConverter{coins: map[model.Coin]string{
		model.BTC: "BTCEUR",
	}}
}

// CoinConverter converts from the internal coin representation to binance specific model
type CoinConverter struct {
	coins map[model.Coin]string
}

// Pair transforms the internal coin type to an exchange traded pair.
func (c CoinConverter) Pair(p model.Coin) string {
	if coin, ok := c.coins[p]; ok {
		return coin
	}
	return string(p)
}

// Coin transforms the binance coin representation to the internal coin types.
func (c CoinConverter) Coin(p string) model.Coin {
	for coin, pair := range c.coins {
		if pair == p {
			return coin
		}
	}
	return model.Coin(p)
}

// Type creates a new type converter for binance.
func Type() TypeConverter {
	return TypeConverter{types: map[model.Type]binance.SideType{
		model.Buy:  binance.SideTypeBuy,
		model.Sell: binance.SideTypeSell,
	}}
}

// TypeConverter converts between binance and internal model types.
type TypeConverter struct {
	types map[model.Type]binance.SideType
}

// To transforms from a binance type representation to the internal coin model.
func (t TypeConverter) To(s binance.SideType) model.Type {
	for t, ts := range t.types {
		if ts == s {
			return t
		}
	}
	log.Error().Str("type", string(s)).Msg("unexpected type")
	return model.NoType
}

// From transforms from the internal model representation to the binance model.
func (t TypeConverter) From(s model.Type) binance.SideType {
	if ts, ok := t.types[s]; ok {
		return ts
	}
	log.Error().Str("type", string(s)).Msg("unexpected type")
	return ""
}
