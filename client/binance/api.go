package binance

import (
	"context"

	"github.com/drakos74/free-coin/internal/account"

	"github.com/adshao/go-binance/v2"
	"github.com/drakos74/free-coin/client/binance/model"
	coinmodel "github.com/drakos74/free-coin/internal/model"
)

func createAPI(user account.Name) exchange {
	k, s := exchangeConfig(user)
	client := binance.NewClient(k, s)
	return newBinanceAPI(client)
}

type exchange interface {
	GetExchangeInfo() (res *binance.ExchangeInfo, err error)
	ListPrice() (res []*binance.SymbolPrice, err error)
	CreateOrder(converter model.Converter, order coinmodel.TrackedOrder, volume string) (res *binance.CreateOrderResponse, err error)
	GetAccount() (res *binance.Account, err error)
	GetMarginAccount() (res *binance.MarginAccount, err error)
	GetMarginPairs() (res []*binance.MarginAllPair, err error)
}

type binanceAPI struct {
	client *binance.Client
}

func newBinanceAPI(client *binance.Client) *binanceAPI {
	return &binanceAPI{client: client}
}

func (b *binanceAPI) GetExchangeInfo() (res *binance.ExchangeInfo, err error) {
	return b.client.NewExchangeInfoService().Do(context.Background())
}

func (b *binanceAPI) ListPrice() (res []*binance.SymbolPrice, err error) {
	return b.client.NewListPricesService().Do(context.Background())
}

func (b *binanceAPI) GetAccount() (res *binance.Account, err error) {
	return b.client.NewGetAccountService().Do(context.Background())
}

func (b *binanceAPI) GetMarginAccount() (res *binance.MarginAccount, err error) {
	return b.client.NewGetMarginAccountService().Do(context.Background())
}

func (b *binanceAPI) GetMarginPairs() (res []*binance.MarginAllPair, err error) {
	return b.client.NewGetMarginAllPairsService().Do(context.Background())
}

func (b *binanceAPI) CreateOrder(converter model.Converter, order coinmodel.TrackedOrder, volume string) (res *binance.CreateOrderResponse, err error) {
	if order.Leverage > 0 {
		return b.client.NewCreateMarginOrderService().
			Symbol(converter.Coin.Pair(order.Coin)).
			Side(converter.Type.From(order.Type)).
			Type(converter.OrderType.From(order.OType)).
			//TimeInForce(binance.TimeInForceTypeGTC).
			Quantity(volume).
			//Price(coinmodel.Price.Format(order.Coin, order.Volume)).
			Do(context.Background())
	} else {
		return b.client.NewCreateOrderService().
			Symbol(converter.Coin.Pair(order.Coin)).
			Side(converter.Type.From(order.Type)).
			Type(converter.OrderType.From(order.OType)).
			//TimeInForce(binance.TimeInForceTypeGTC).
			Quantity(volume).
			//Price(coinmodel.Price.Format(order.Coin, order.Volume)).
			Do(context.Background())
	}
}
