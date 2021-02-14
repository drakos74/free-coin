package model

import (
	"fmt"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/coinapi"
	"github.com/rs/zerolog/log"
)

const (
	XLINKZEUR  = "LINKEUR"
	XWAVESZEUR = "WAVESEUR"
	XDOTZEUR   = "DOTEUR"
	XXRPZEUR   = "XXRPZEUR"
)

func Pair(p coinapi.Coin) string {
	switch p {
	case coinapi.BTC:
		return krakenapi.XXBTZEUR
	case coinapi.ETH:
		return krakenapi.XETHZEUR
	case coinapi.EOS:
		return krakenapi.EOSEUR
	case coinapi.LINK:
		return XLINKZEUR
	case coinapi.WAVES:
		return XWAVESZEUR
	case coinapi.DOT:
		return XDOTZEUR
	case coinapi.XRP:
		return XXRPZEUR
	default:
		panic(fmt.Sprintf("unknown coin: %d", p))
	}
}

func Coin(p string) coinapi.Coin {
	switch p {
	case krakenapi.XXBTZEUR:
		return coinapi.BTC
	case krakenapi.XETHZEUR:
		return coinapi.ETH
	case krakenapi.EOSEUR:
		return coinapi.EOS
	case XLINKZEUR:
		return coinapi.LINK
	case XWAVESZEUR:
		return coinapi.WAVES
	case XDOTZEUR:
		return coinapi.DOT
	case XXRPZEUR:
		return coinapi.XRP
	default:
		return coinapi.NoCoin
	}
}

func Type(s string) coinapi.Type {
	switch s {
	case "buy":
		return coinapi.Buy
	case "sell":
		return coinapi.Sell
	default:
		log.Error().Str("type", s).Msg("unexpected type")
		return coinapi.NoType
	}
}

var NullResponse = &krakenapi.TradesResponse{
	Trades: []krakenapi.TradeInfo{
		{},
	},
}
