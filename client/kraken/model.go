package kraken

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/api"

	krakenapi "github.com/beldur/kraken-go-api-client"
	coinmodel "github.com/drakos74/free-coin/internal/model"
)

const (
	Name api.ExchangeName = "kraken"
)

// newTrade creates a new trade from the kraken trade response.
func (r *RemoteClient) newTrade(pair string, active bool, live bool, trade krakenapi.TradeInfo) coinmodel.Trade {
	var t coinmodel.Type
	if trade.Buy {
		t = coinmodel.Buy
	} else if trade.Sell {
		t = coinmodel.Sell
	}
	return coinmodel.Trade{
		Coin:   r.converter.Coin.Coin(pair),
		Price:  trade.PriceFloat,
		Volume: trade.VolumeFloat,
		Time:   time.Unix(trade.Time, 0),
		Active: active,
		Live:   live,
		Type:   t,
	}
}

// newOrder creates a new order from a kraken order description.
// TODO : find out why this fails ... e.g. orderdescription is empty
func (r *RemoteExchange) newOrder(order krakenapi.OrderDescription) *coinmodel.Order {
	fmt.Printf("order-description-from-kraken = %+v\n", order)
	//_, err := strconv.ParseFloat(order.PrimaryPrice, 64)
	//if err != nil {
	//	log.Error().Err(err).Str("price", order.PrimaryPrice).Msg("could not read price")
	//}
	return nil
}

// newPosition creates a new position based on the kraken position response.
func (r *RemoteExchange) newPosition(id string, response krakenapi.Position) coinmodel.Position {
	net := float64(response.Net)
	fees := response.Fee * 2
	return coinmodel.Position{
		Data: coinmodel.Data{
			ID:      id,
			TxID:    response.TransactionID,
			OrderID: response.OrderTransactionID,
		},
		MetaData: coinmodel.MetaData{
			OpenTime: time.Unix(int64(response.TradeTime), 0),
			Net:      net,
			Fees:     fees,
			Cost:     response.Cost,
		},
		Coin:         r.converter.Coin.Coin(response.Pair),
		Type:         r.converter.Type.To(response.PositionType),
		OpenPrice:    response.Cost / response.Volume,
		CurrentPrice: response.Value / response.Volume,
		Volume:       response.Volume,
	}
}
