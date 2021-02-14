package private

import (
	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/coinapi"
	"github.com/drakos74/free-coin/kraken/api"
)

func NewPosition(id string, response krakenapi.Position) coinapi.Position {
	net := api.Net(response.Net)
	fees := response.Fee * 2
	return coinapi.Position{
		ID:           id,
		Coin:         api.Coin(response.Pair),
		Type:         api.Type(response.PositionType),
		OpenPrice:    response.Cost / response.Volume,
		CurrentPrice: response.Value / response.Volume,
		Cost:         response.Cost,
		Net:          net,
		Fees:         fees,
		Volume:       response.Volume,
		Trigger:      coinapi.Action{},
	}
}
