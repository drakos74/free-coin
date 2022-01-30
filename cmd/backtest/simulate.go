package main

import (
	"github.com/drakos74/free-coin/internal/algo/processor/ml"
	"github.com/drakos74/free-coin/internal/model"
)

func simulate(signals []*ml.Signal, threshold int) Result {

	result := Result{
		Threshold: threshold,
	}

	status := model.NoType

	for _, signal := range signals {
		if signal.Filter(threshold) {

			switch signal.Type {
			case model.Buy:
				if status == model.NoType || status == model.Sell {
					result.Value -= signal.Price
					result.Coins++
					status = signal.Type
					result.CoinValue = signal.Price
					result.Trades++
					result.Fees += signal.Price / 1000
				}
			case model.Sell:
				if status == model.NoType || status == model.Buy {
					result.Value += signal.Price
					result.Coins--
					status = signal.Type
					result.CoinValue = signal.Price
					result.Trades++
					result.Fees += signal.Price / 1000
				}
			}
		}
	}

	return result

}
