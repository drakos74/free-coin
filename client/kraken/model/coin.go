package model

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/api"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/rs/zerolog/log"
)

const (
	XLINKZEUR  = "LINKEUR"
	XWAVESZEUR = "WAVESEUR"
	XDOTZEUR   = "DOTEUR"
	XXRPZEUR   = "XXRPZEUR"
)

func Pair(p api.Coin) string {
	switch p {
	case api.BTC:
		return krakenapi.XXBTZEUR
	case api.ETH:
		return krakenapi.XETHZEUR
	case api.EOS:
		return krakenapi.EOSEUR
	case api.LINK:
		return XLINKZEUR
	case api.WAVES:
		return XWAVESZEUR
	case api.DOT:
		return XDOTZEUR
	case api.XRP:
		return XXRPZEUR
	default:
		panic(fmt.Sprintf("unknown coin: %s", p))
	}
}

func Coin(p string) api.Coin {
	switch p {
	case krakenapi.XXBTZEUR:
		return api.BTC
	case krakenapi.XETHZEUR:
		return api.ETH
	case krakenapi.EOSEUR:
		return api.EOS
	case XLINKZEUR:
		return api.LINK
	case XWAVESZEUR:
		return api.WAVES
	case XDOTZEUR:
		return api.DOT
	case XXRPZEUR:
		return api.XRP
	default:
		return api.NoCoin
	}
}

func Type(s string) api.Type {
	switch s {
	case "buy":
		return api.Buy
	case "sell":
		return api.Sell
	default:
		log.Error().Str("type", s).Msg("unexpected type")
		return api.NoType
	}
}

var NullResponse = &krakenapi.TradesResponse{
	Trades: []krakenapi.TradeInfo{
		{},
	},
}
