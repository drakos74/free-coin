package external

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/api"

	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

func addTargets(client api.Exchange, grafana *metrics.Server, registry storage.Registry) {
	registryPath := filepath.Join(storage.DefaultDir, storage.RegistryDir, storage.SignalsPath)
	// TODO : get current prices ...

	grafana.Target("PnL", readFromRegistry(client, registryPath, registry, noError, addPnL))

	grafana.Target("trades", readFromRegistry(client, registryPath, registry, noError, count))

	grafana.Target("errors", readFromRegistry(client, registryPath, registry, isError, count))
}

func readFromRegistry(client api.Exchange, registryPath string, registry storage.Registry, condition func(dir string) bool, addSeries func(index string, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series) func(data map[string]interface{}) []metrics.Series {
	return func(data map[string]interface{}) []metrics.Series {
		assets := make([]metrics.Series, 0)
		accountFor := make(map[string]struct{})

		prices, err := client.CurrentPrice(context.Background())

		if err != nil {
			prices = make(map[model.Coin]model.CurrentPrice)
		}

		err = filepath.Walk(registryPath, func(path string, info os.FileInfo, err error) error {
			// take into account only files
			dir := filepath.Dir(path)
			index := filepath.Base(dir)
			if info != nil && !info.IsDir() {
				if _, ok := accountFor[index]; ok {
					return nil
				}
				series, err := parseEvents(dir, index, registry, prices, condition, addSeries)
				if err == nil {
					assets = append(assets, series)
					accountFor[index] = struct{}{}
				}
			}
			return nil
		})
		if err != nil {
			log.Error().
				Str("path", registryPath).
				Err(err).
				Msg("could not look into registry path")
		}
		return assets
	}
}

func parseEvents(dir string, index string, registry storage.Registry, prices map[model.Coin]model.CurrentPrice, condition func(dir string) bool, addSeries func(index string, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series) (metrics.Series, error) {
	if condition(dir) {

		orders := []Order{{}}
		// look up one level up
		key := storage.K{
			Pair:  filepath.Base(filepath.Dir(dir)),
			Label: index,
		}
		err := registry.GetAll(key, &orders)
		if err != nil {
			return metrics.Series{}, fmt.Errorf("could not read from registry: %w", err)
		}
		var sortedOrders Orders
		sortedOrders = orders
		sort.Sort(sortedOrders)

		return addSeries(index, prices, sortedOrders), nil
	}
	return metrics.Series{}, fmt.Errorf("invalid dir")
}

func addPnL(index string, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series {
	series := metrics.Series{
		Target:     index,
		DataPoints: make([][]float64, 0),
	}
	sum := 0.0
	var close bool
	var lastOrder Order
	for _, order := range orders {
		switch order.Order.Type {
		case model.Buy:
			sum -= order.Order.Price * order.Order.Volume
		case model.Sell:
			sum += order.Order.Price * order.Order.Volume
		}
		// every second order is a closing one ...
		if close {
			series.DataPoints = append(series.DataPoints, []float64{sum, float64(cointime.ToMilli(order.Order.Time))})
		}
		lastOrder = order
		close = !close
	}
	// if we are at the last one .. we ll add a virtual one at the current price
	if close {
		if p, ok := prices[lastOrder.Order.Coin]; ok {
			// if we have a last price for this asset ...
			var diff float64
			switch lastOrder.Order.Type {
			case model.Buy:
				diff = p.Price - lastOrder.Order.Price
			case model.Sell:
				diff = lastOrder.Order.Price - p.Price
			}
			sum += lastOrder.Order.Volume * diff
			series.DataPoints = append(series.DataPoints, []float64{sum, float64(cointime.ToMilli(time.Now()))})
		}
	}
	return series
}

func count(index string, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series {
	count := 0.0

	series := metrics.Series{
		Target:     index,
		DataPoints: make([][]float64, 0),
	}
	for _, order := range orders {
		count++
		series.DataPoints = append(series.DataPoints, []float64{count, float64(cointime.ToMilli(order.Order.Time))})
	}
	return series
}

func instance(index string, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series {
	series := metrics.Series{
		Target:     index,
		DataPoints: make([][]float64, 0),
	}
	for _, order := range orders {
		series.DataPoints = append(series.DataPoints, []float64{1.0, float64(cointime.ToMilli(order.Order.Time))})
	}
	return series
}

func noError(dir string) bool {
	return !isError(dir)
}

func isError(dir string) bool {
	return strings.HasSuffix(dir, "error")
}
