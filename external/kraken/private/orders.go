package private

import (
	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/coinapi"
	"github.com/drakos74/free-coin/kraken/api"
)

func NewOrder(id string, order krakenapi.Order) coinapi.Order {
	return coinapi.Order{
		ID:       id,
		Coin:     api.Coin(order.Description.AssetPair),
		Type:     api.Type(order.Description.Type),
		OType:    api.OrderType(order.Description.OrderType),
		Volume:   api.Volume(order.Volume),
		Leverage: api.Leverage(order.Description.Leverage),
		Price:    order.Price,
	}
}

func FromDescription(id string, order krakenapi.OrderDescription) *coinapi.Order {
	return &coinapi.Order{
		ID:       id,
		Coin:     api.Coin(order.AssetPair),
		Type:     api.Type(order.Type),
		OType:    api.OrderType(order.OrderType),
		Leverage: api.Leverage(order.Leverage),
		Price:    api.Price(order.PrimaryPrice),
	}
}
