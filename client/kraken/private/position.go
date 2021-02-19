package private

import (
	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/internal/api"
)

// NewPosition creates a new position based on the kraken position response.
func NewPosition(id string, response krakenapi.Position) api.Position {
	net := float64(response.Net)
	fees := response.Fee * 2
	return api.Position{
		ID:           id,
		Coin:         api.Coin(response.Pair),
		Type:         api.Type(response.PositionType),
		OpenPrice:    response.Cost / response.Volume,
		CurrentPrice: response.Value / response.Volume,
		Cost:         response.Cost,
		Net:          net,
		Fees:         fees,
		Volume:       response.Volume,
	}
}
