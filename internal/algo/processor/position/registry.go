package position

import (
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	jsonstore "github.com/drakos74/free-coin/internal/storage/file/json"
)

func GetOpen(registryKeyDir string, coin string) ([]coinmodel.TrackedOrder, error) {
	registry := jsonstore.NewEventRegistry(registryKeyDir)

	// get the events from the trade processor
	orders := []coinmodel.TrackedOrder{{}}
	err := registry.GetAll(storage.K{
		Pair:  coin,
		Label: OpenPositionRegistryKey,
	}, &orders)
	return orders, err
}

func GetClosed(registryKeyDir string, coin string) ([]coinmodel.TrackedPosition, error) {
	registry := jsonstore.NewEventRegistry(registryKeyDir)

	// get the events from the trade processor
	positions := []coinmodel.TrackedPosition{{}}
	err := registry.GetAll(storage.K{
		Pair:  coin,
		Label: ClosePositionRegistryKey,
	}, &positions)
	return positions, err
}
