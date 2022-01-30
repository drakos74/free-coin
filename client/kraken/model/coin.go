package model

import (
	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	XLINKZEUR  = "LINKEUR"
	XWAVESZEUR = "WAVESEUR"
	XDOTZEUR   = "DOTEUR"
	XXRPZEUR   = "XXRPZEUR"
	MINAZEUR   = "MINAZEUR"

	SOLZEUR = "SOLEUR"
	KSMZEUR = "KSMEUR"

	KAVAZEUR  = "KAVAEUR"
	AAVEZEUR  = "AAVEEUR"
	MATICZEUR = "MATICEUR"

	DAIZEUR  = "DAIEUR"
	TRXZEUR  = "TRXEUR"
	XLMZEUR  = "XLMEUR"
	FILZEUR  = "FILEUR"
	XMRZEUR  = "XMREUR"
	XTZZEUR  = "XTZXEUR"
	FLOWZEUR = "FLOWEUR"
	SCZEUR   = "SCEUR"
	KEEPZEUR = "KEEPEUR"
	REPZEUR  = "REPEUR"
)

// Coin creates a new coin converter for kraken.
func Coin() CoinConverter {
	return CoinConverter{coins: map[model.Coin]string{
		model.BTC:   krakenapi.XXBTZEUR,
		model.ETH:   krakenapi.XETHZEUR,
		model.EOS:   krakenapi.EOSEUR,
		model.ADA:   krakenapi.ADAEUR,
		model.LINK:  XLINKZEUR,
		model.WAVES: XWAVESZEUR,
		model.DOT:   XDOTZEUR,
		model.XRP:   XXRPZEUR,
		model.MINA:  MINAZEUR,
		model.SOL:   SOLZEUR,
		model.KSM:   KSMZEUR,
		model.KAVA:  KAVAZEUR,
		model.AAVE:  AAVEZEUR,
		model.MATIC: MATICZEUR,
		model.DAI:   DAIZEUR,
		model.TRX:   TRXZEUR,
		model.XLM:   XLMZEUR,
		model.FIL:   FILZEUR,
		model.XMR:   XMRZEUR,
		model.XTZ:   XTZZEUR,
		model.FLOW:  FLOWZEUR,
		model.SC:    SCZEUR,
		model.KEEP:  KEEPZEUR,
		model.REP:   REPZEUR,
	}}
}

// CoinConverter converts from the internal coin representation to kraken specific model
type CoinConverter struct {
	coins map[model.Coin]string
}

// Pair transforms the internal coin type to an exchange traded pair.
func (c CoinConverter) Pair(p model.Coin) string {
	if coin, ok := c.coins[p]; ok {
		return coin
	}
	log.Debug().Str("coin", string(p)).Msg("unknown coin")
	return ""
}

// Coin transforms the kraken coin representation to the internal coin types.
func (c CoinConverter) Coin(p string) model.Coin {
	for coin, pair := range c.coins {
		if pair == p {
			return coin
		}
	}
	log.Debug().Str("pair", p).Msg("unknown coin")
	return model.NoCoin
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
