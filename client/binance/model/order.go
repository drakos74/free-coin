package model

import (
	"fmt"

	"github.com/adshao/go-binance/v2"

	"github.com/drakos74/free-coin/internal/model"
)

// Leverage creates a new leverage converter for kraken.
func Leverage() LeverageConverter {
	return LeverageConverter{
		l:   map[model.Leverage]string{},
		max: map[model.Coin]model.Leverage{},
	}
}

// LeverageConverter converts from the kraken leverage definitions to the internal leverage requirements.
type LeverageConverter struct {
	l   map[model.Leverage]string
	max map[model.Coin]model.Leverage
}

// For returns the leverage for the given order.
func (lv LeverageConverter) For(order model.Order) string {
	leverage := order.Leverage
	if leverage > lv.max[order.Coin] {
		leverage = lv.max[order.Coin]
	}
	if s, ok := lv.l[leverage]; ok {
		return s
	}
	return "1:1"
}

// From converts te leverage from the kraken model to the internal leverage model.
func (lv LeverageConverter) From(s string) model.Leverage {
	for l, leverage := range lv.l {
		if leverage == s {
			return l
		}
	}
	return model.NoLeverage
}

// OrderType creates an order type converter
func OrderType() OrderTypeConverter {
	return OrderTypeConverter{
		orderTypes: map[model.OrderType]binance.OrderType{
			model.Market:     binance.OrderTypeMarket,
			model.StopLoss:   binance.OrderTypeStopLoss,
			model.TakeProfit: binance.OrderTypeTakeProfit,
			model.Limit:      binance.OrderTypeLimit,
		},
	}
}

// OrderTypeConverter converts from a kraken order type model to the internal one.
type OrderTypeConverter struct {
	orderTypes map[model.OrderType]binance.OrderType
}

// From translates the type of order to kraken specific representation.
func (ot OrderTypeConverter) From(t model.OrderType) binance.OrderType {
	if orderType, ok := ot.orderTypes[t]; ok {
		return orderType
	}
	panic(fmt.Sprintf("unkown order type %v", t))
}

// To translates the type of the kraken order to the internal representation.
func (ot OrderTypeConverter) To(orderType binance.OrderType) model.OrderType {
	for t, ordT := range ot.orderTypes {
		if orderType == ordT {
			return t
		}
	}
	panic(fmt.Sprintf("unkown order type %v", orderType))
}
