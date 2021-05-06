package model

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/adshao/go-binance/v2"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
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

type LotSizeFilter struct {
	Type        string `json:"filterType"`
	MaxQuantity string `json:"maxQty"`
	MinQuantity string `json:"minQty"`
	StepSize    string `json:"stepSize"`
}

type LotSize struct {
	Type        string
	MaxQuantity float64
	MinQuantity float64
	StepSize    float64
}

func NewLotSize(filter LotSizeFilter) (LotSize, error) {
	lotSize := LotSize{}
	max, err := strconv.ParseFloat(filter.MaxQuantity, 64)
	if err != nil {
		return lotSize, fmt.Errorf("could not parse max-quantity: %w", err)
	}
	lotSize.MaxQuantity = max
	min, err := strconv.ParseFloat(filter.MinQuantity, 64)
	if err != nil {
		return lotSize, fmt.Errorf("could not parse min-quantity: %w", err)
	}
	lotSize.MinQuantity = min
	step, err := strconv.ParseFloat(filter.StepSize, 64)
	if err != nil {
		return lotSize, fmt.Errorf("could not parse step-size: %w", err)
	}
	lotSize.StepSize = step
	return lotSize, nil
}

func (l LotSize) Adjust(volume float64) float64 {
	if volume < l.MinQuantity {
		return l.MinQuantity
	}
	if volume > l.MaxQuantity {
		return l.MaxQuantity
	}
	// adjust the min step
	steps := volume / l.StepSize
	// note steps must be round integer
	// pick the floor to avoid insufficient funds for closing positions and sell-off
	return math.Floor(steps) * l.StepSize
}

func ParseLOTSize(filters []map[string]interface{}) (LotSize, error) {

	for _, f := range filters {
		b, err := json.Marshal(f)
		if err != nil {
			log.Trace().Err(err).Msg("could not encode filter")
			continue
		}
		var lotSizeFilter LotSizeFilter
		err = json.Unmarshal(b, &lotSizeFilter)
		if err != nil {
			log.Trace().Err(err).Str("map", fmt.Sprintf("%+v", f)).Msg("could not parse as lot size")
			continue
		}

		if lotSizeFilter.Type == "LOT_SIZE" {
			return NewLotSize(lotSizeFilter)
		} else {
			log.Trace().Str("filter", fmt.Sprintf("%+v", lotSizeFilter)).Msg("found other")
		}

	}

	return LotSize{}, fmt.Errorf("could not find lot size in: %+v", filters)
}
