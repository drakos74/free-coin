package external

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

func addTargets(grafana *metrics.Server, registry storage.Registry) {
	registryPath := filepath.Join(storage.DefaultDir, storage.RegistryDir, storage.SignalsPath)
	// TODO : get current prices ...

	grafana.Target("PnL", readFromRegistry(registryPath, registry, noError, addPnL))

	grafana.Target("trades", readFromRegistry(registryPath, registry, noError, count))

	grafana.Target("errors", readFromRegistry(registryPath, registry, isError, count))
}

func readFromRegistry(registryPath string, registry storage.Registry, condition func(dir string) bool, addSeries func(index string, orders Orders, assets []metrics.Series)) func(data map[string]interface{}) []metrics.Series {
	return func(data map[string]interface{}) []metrics.Series {
		assets := make([]metrics.Series, 0)
		err := filepath.Walk(registryPath, func(path string, info os.FileInfo, err error) error {
			// take into account only files
			if info != nil && !info.IsDir() {
				return parseEvents(path, registry, assets, condition, addSeries)
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

func parseEvents(path string, registry storage.Registry, assets []metrics.Series, condition func(dir string) bool, addSeries func(index string, orders Orders, assets []metrics.Series)) error {
	accountFor := make(map[string]struct{})

	dir := filepath.Dir(path)
	index := filepath.Base(dir)
	if condition(dir) {
		if _, ok := accountFor[index]; ok {
			return nil
		}
		orders := []Order{{}}
		// look up one level up
		key := storage.K{
			Pair:  filepath.Base(filepath.Dir(dir)),
			Label: index,
		}
		err := registry.GetAll(key, &orders)
		if err != nil {
			log.Error().Str("key", fmt.Sprintf("%+v", key)).Err(err).Msg("could not parse orders")
		}
		var sortedOrders Orders
		sortedOrders = orders
		sort.Sort(sortedOrders)

		addSeries(index, sortedOrders, assets)
		accountFor[index] = struct{}{}
	}
	return nil
}

func addPnL(index string, orders Orders, assets []metrics.Series) {
	series := metrics.Series{
		Target:     index,
		DataPoints: make([][]float64, 0),
	}
	sum := 0.0
	var close bool
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
		close = !close
	}
	assets = append(assets, series)
}

func count(index string, orders Orders, assets []metrics.Series) {
	count := 0.0

	series := metrics.Series{
		Target:     index,
		DataPoints: make([][]float64, 0),
	}
	for _, order := range orders {
		count++
		series.DataPoints = append(series.DataPoints, []float64{count, float64(cointime.ToMilli(order.Order.Time))})
	}
	assets = append(assets, series)
}

func noError(dir string) bool {
	return !isError(dir)
}

func isError(dir string) bool {
	return strings.HasSuffix(dir, "error")
}
