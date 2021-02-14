package api

import (
	"fmt"
	"strconv"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/rs/zerolog/log"
)

const (
	XLINKZEUR  = "LINKEUR"
	XWAVESZEUR = "WAVESEUR"
	XDOTZEUR   = "DOTEUR"
	XXRPZEUR   = "XXRPZEUR"
)

func Pair(p model.Coin) string {
	switch p {
	case model.BTC:
		return krakenapi.XXBTZEUR
	case model.ETH:
		return krakenapi.XETHZEUR
	case model.EOS:
		return krakenapi.EOSEUR
	case model.LINK:
		return XLINKZEUR
	case model.WAVES:
		return XWAVESZEUR
	case model.DOT:
		return XDOTZEUR
	case model.XRP:
		return XXRPZEUR
	default:
		panic(fmt.Sprintf("unknown coin: %d", p))
	}
}

func Coin(p string) model.Coin {
	switch p {
	case krakenapi.XXBTZEUR:
		return model.BTC
	case krakenapi.XETHZEUR:
		return model.ETH
	case krakenapi.EOSEUR:
		return model.EOS
	case XLINKZEUR:
		return model.LINK
	case XWAVESZEUR:
		return model.WAVES
	case XDOTZEUR:
		return model.DOT
	case XXRPZEUR:
		return model.XRP
	default:
		return model.NoCoin
	}
}

func Type(s string) model.Type {
	switch s {
	case "buy":
		return model.Buy
	case "sell":
		return model.Sell
	default:
		log.Error().Str("type", s).Msg("unexpected type")
		return model.NoType
	}
}

var orderTypes = map[model.OrderType]string{
	model.Market:     "market",
	model.Limit:      "limit",
	model.StopLoss:   "stop-loss",
	model.TakeProfit: "take-profit",
}

func OrderType(s string) model.OrderType {
	for k, v := range orderTypes {
		if v == s {
			return k
		}
	}
	log.Error().Str("order type", s).Msg("unexpected order type")
	return model.NoOrderType
}

var leverage = map[model.Leverage]string{
	model.L_5: "5:1",
	model.L_3: "3:1",
}

func LeverageString(l model.Leverage) string {
	if lv, ok := leverage[l]; ok {
		return lv
	}
	panic(fmt.Sprintf("cannot find leverage definition for '%v' in types '%+v'", l, leverage))
}

func Leverage(s string) model.Leverage {
	for k, v := range leverage {
		if v == s {
			return k
		}
	}
	panic(fmt.Sprintf("could not match leverage '%s' to available types '%+v'", s, orderTypes))
}

func Volume(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(fmt.Sprintf("could not parse volume '%s': %v", s, err))
	}
	return v
}

func Price(s string) float64 {
	p, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(fmt.Sprintf("could not parse price '%s': %v", s, err))
	}
	return p
}

func Net(s krakenapi.Float64) float64 {
	return float64(s)
}

var NullResponse = &krakenapi.TradesResponse{
	Trades: []krakenapi.TradeInfo{
		{},
	},
}
