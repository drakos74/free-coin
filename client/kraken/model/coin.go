package model

import (
	ws "github.com/aopoltorzhicky/go_kraken/websocket"
	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	XBTCZEUR = "XBTEUR"
	XETHZEUR = "ETHEUR"

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

type ApiPair struct {
	Altname string
	AltPair string
	Rest    string
	Socket  string
}

// Coin creates a new coin converter for kraken.
func Coin() CoinConverter {
	return CoinConverter{coins: map[model.Coin]ApiPair{
		model.BTC: {
			Altname: "XBTEUR",
			AltPair: "XBT/EUR",
			Rest:    krakenapi.XXBTZEUR,
			Socket:  ws.BTCEUR,
		},
		model.ETH: {
			Altname: "ETHEUR",
			Rest:    krakenapi.XETHZEUR,
			Socket:  ws.ETHEUR,
		},
		model.EOS: {
			Altname: "EOSEUR",
			Rest:    krakenapi.EOSEUR,
			Socket:  ws.EOSEUR,
		},
		model.ADA: {
			Altname: "ADAEUR",
			Rest:    krakenapi.ADAEUR,
			Socket:  ws.ADAEUR,
		},
		model.LINK: {
			Altname: "LINKEUR",
			Rest:    XLINKZEUR,
			Socket:  "LINK/EUR",
		},
		model.WAVES: {
			Altname: "WAVESEUR",
			Rest:    XWAVESZEUR,
			Socket:  "WAVES/EUR",
		},
		model.DOT: {
			Altname: "DOTEUR",
			Rest:    XDOTZEUR,
			Socket:  ws.DOTEUR,
		},
		model.XRP: {
			Altname: "XRPEUR",
			Rest:    XXRPZEUR,
			Socket:  ws.XRPEUR,
		},
		model.MINA: {
			Altname: "MINAEUR",
			Rest:    MINAZEUR,
			Socket:  "MINA/EUR",
		},
		model.SOL: {
			Altname: "SOLEUR",
			Rest:    SOLZEUR,
			Socket:  "SOL/EUR",
		},
		model.KSM: {
			Altname: "KSMEUR",
			Rest:    KSMZEUR,
			Socket:  "KSM/EUR",
		},
		model.KAVA: {
			Altname: "KAVAEUR",
			Rest:    KAVAZEUR,
			Socket:  "KAVA/EUR",
		},
		model.AAVE: {
			Altname: "AAVEEUR",
			Rest:    AAVEZEUR,
			Socket:  "AAVE/EUR",
		},
		model.MATIC: {
			Altname: "MATICEUR",
			Rest:    MATICZEUR,
			Socket:  "MATIC/EUR",
		},
		model.DAI: {
			Altname: "DAIEUR",
			Rest:    DAIZEUR,
			Socket:  "DAI/EUR",
		},
		model.TRX: {
			Altname: "TRXEUR",
			Rest:    TRXZEUR,
			Socket:  "TRX/EUR",
		},
		model.XLM: {
			Altname: "XLMEUR",
			Rest:    XLMZEUR,
			Socket:  ws.XLMEUR,
		},
		model.FIL: {
			Rest:   FILZEUR,
			Socket: "FIL/EUR",
		},
		model.XMR: {
			Altname: "FILEUR",
			Rest:    XMRZEUR,
			Socket:  ws.XMREUR,
		},
		model.XTZ: {
			Altname: "XTZEUR",
			Rest:    XTZZEUR,
			Socket:  ws.XTZEUR,
		},
		model.FLOW: {
			Altname: "FLOWEUR",
			Rest:    FLOWZEUR,
			Socket:  "FLOW/EUR",
		},
		model.SC: {
			Altname: "SCEUR",
			Rest:    SCZEUR,
			Socket:  "SC/EUR",
		},
		model.KEEP: {
			Altname: "KEEPEUR",
			Rest:    KEEPZEUR,
			Socket:  "KEEP/EUR",
		},
		model.REP: {
			Altname: "REPEUR",
			Rest:    REPZEUR,
			Socket:  ws.REPEUR,
		},
	}}
}

// CoinConverter converts from the internal coin representation to kraken specific model
type CoinConverter struct {
	coins map[model.Coin]ApiPair
}

// Alt transforms the internal coin type to an exchange traded pair.
func (c CoinConverter) Alt(p string) (ApiPair, bool) {
	for _, pair := range c.coins {
		if pair.Altname == p {
			return pair, true
		}
	}
	return ApiPair{}, false
}

// Pair transforms the internal coin type to an exchange traded pair.
func (c CoinConverter) Pair(p model.Coin) (ApiPair, bool) {
	if coin, ok := c.coins[p]; ok {
		return coin, true
	}
	//log.Debug().Str("coin", string(p)).Msg("unknown coin")
	return ApiPair{}, false
}

// Coin transforms the kraken coin representation to the internal coin types.
func (c CoinConverter) Coin(p string) model.Coin {
	for coin, pair := range c.coins {
		if pair.Rest == p || pair.Socket == p || pair.AltPair == p {
			return coin
		}
	}
	//log.Debug().Str("pair", p).Msg("unknown coin")
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
		if ts == s || ts[0:1] == s {
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
