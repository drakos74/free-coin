package trade

import (
	"github.com/drakos74/free-coin/internal/storage"
	jsonstore "github.com/drakos74/free-coin/internal/storage/file/json"
)

func GetPairs(registryKeyDir string, coin string) ([]PredictionPair, error) {
	registry := jsonstore.NewEventRegistry(registryKeyDir)
	// get the events from the trade processor
	predictionPairs := []PredictionPair{{}}
	err := registry.GetAll(storage.K{
		Pair:  coin,
		Label: ProcessorName,
	}, &predictionPairs)
	return predictionPairs, err
}

func StrategyEvents(registryKeyDir string, coin string) ([]StrategyEvent, error) {
	registry := jsonstore.NewEventRegistry(registryKeyDir)
	// get the events from the trade processor
	events := []StrategyEvent{{}}
	err := registry.GetAll(strategyKey(coin), &events)
	return events, err
}
