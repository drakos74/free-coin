package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/drakos74/free-coin/client/binance/model"
	coinmodel "github.com/drakos74/free-coin/internal/model"

	"github.com/adshao/go-binance/v2"
)

func TestExchange_Balance(t *testing.T) {

	exchange := &Exchange{
		api: testAPI{
			prices:  "testdata/responses/prices.json",
			account: "testdata/responses/balance.json",
		},
		converter: model.NewConverter(),
	}

	prices, err := exchange.CurrentPrice(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1428, len(prices))

	bb, err := exchange.Balance(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, 14, len(bb))

	for _, b := range bb {
		if price, ok := prices[b.Coin]; ok {
			assert.Equal(t, price.Price, b.Price)
		} else {
			assert.Contains(t, []string{"INSUSDT", "BETHUSDT", "USDTUSDT"}, string(b.Coin))
		}
	}
}

type testAPI struct {
	prices  string
	account string
}

func (t testAPI) GetExchangeInfo() (res *binance.ExchangeInfo, err error) {
	panic("implement me")
}

func (t testAPI) ListPrice() (res []*binance.SymbolPrice, err error) {
	if t.prices == "" {
		return nil, fmt.Errorf("could not get price")
	}
	bb, err := ioutil.ReadFile(t.prices)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}
	err = json.Unmarshal(bb, &res)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal file: %w", err)
	}
	return
}

func (t testAPI) CreateOrder(converter model.Converter, order coinmodel.TrackedOrder, volume string) (res *binance.CreateOrderResponse, err error) {
	panic("implement me")
}

func (t testAPI) GetAccount() (res *binance.Account, err error) {
	if t.account == "" {
		return nil, fmt.Errorf("could not get account")
	}
	bb, err := ioutil.ReadFile(t.account)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}
	err = json.Unmarshal(bb, &res)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal file: %w", err)
	}
	return
}

func (t testAPI) GetMarginAccount() (res *binance.MarginAccount, err error) {
	panic("implement me")
}

func (t testAPI) GetMarginPairs() (res []*binance.MarginAllPair, err error) {
	panic("implement me")
}
