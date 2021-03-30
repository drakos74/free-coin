package external

import (
	"context"
	"fmt"
	"math"
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

func readFromRegistry(client api.Exchange, registryPath string, registry storage.Registry, condition func(dir string) bool, addSeries func(index string, timeRange cointime.Range, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series) metrics.TargetQuery {
	return func(data map[string]interface{}, timeRange cointime.Range) []metrics.Series {
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
				series, err := parseEvents(dir, index, timeRange, registry, prices, condition, addSeries)
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

func parseEvents(dir string, index string, timeRange cointime.Range, registry storage.Registry, prices map[model.Coin]model.CurrentPrice, condition func(dir string) bool, addSeries func(index string, timeRange cointime.Range, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series) (metrics.Series, error) {
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

		return addSeries(index, timeRange, prices, sortedOrders), nil
	}
	return metrics.Series{}, fmt.Errorf("invalid dir")
}

func addPnL(index string, timeRange cointime.Range, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series {
	series := metrics.Series{
		Target:     index,
		DataPoints: make([][]float64, 0),
	}
	sum := 0.0
	var open bool
	var lastOrder Order
	var lastSum float64
	openingOrders := make(map[string]Order)
	for _, order := range orders {
		if !timeRange.IsWithin(order.Order.Time) {
			continue
		}
		if order.Order.RefID == "" {
			openingOrders[order.Order.ID] = order
			series.DataPoints = append(series.DataPoints, []float64{lastSum, float64(cointime.ToMilli(order.Order.Time))})
			open = true
		} else {
			// else lets find the opening order
			if o, ok := openingOrders[order.Order.RefID]; ok {
				sum += order.Order.Value() + o.Order.Value()
				series.DataPoints = append(series.DataPoints, []float64{sum, float64(cointime.ToMilli(order.Order.Time))})
			} else {
				log.Error().Str("coin", string(order.Order.Coin)).Str("ref-id", order.Order.RefID).Msg("could not pair order")
			}
			open = false
		}
		lastSum = sum
		lastOrder = order
		//lastSum = sum
		//switch order.Order.Type {
		//case model.Buy:
		//	sum -= order.Order.Price * order.Order.Volume
		//case model.Sell:
		//	sum += order.Order.Price * order.Order.Volume
		//}
		//// every second order is a closing one ...
		//if closingOrder {
		//	series.DataPoints = append(series.DataPoints, []float64{sum, float64(cointime.ToMilli(order.Order.Time))})
		//} else {
		//	series.DataPoints = append(series.DataPoints, []float64{lastSum, float64(cointime.ToMilli(order.Order.Time))})
		//}
		//lastOrder = order
		//closingOrder = !closingOrder
	}
	now := time.Now()
	// if we are at the last one .. we ll add a virtual one at the current price
	log.Info().Str("index", index).Bool("open", open).Time("now", now).Float64("sum", sum).Str("coin", string(lastOrder.Order.Coin)).Msg("pnl")
	if open {
		// we only extrapolate if the time range is close to now
		// and the last order was an opening order
		// and we have a current price for the asset
		if p, ok := prices[lastOrder.Order.Coin]; ok && math.Abs(now.Sub(timeRange.To).Minutes()) < 30 {
			// if we have a last price for this asset ...
			sum += (p.Price - lastOrder.Order.Price) * lastOrder.Order.Volume
			series.DataPoints = append(series.DataPoints, []float64{sum, float64(cointime.ToMilli(now))})
		}
	}
	return series
}

func count(index string, timeRange cointime.Range, prices map[model.Coin]model.CurrentPrice, orders Orders) metrics.Series {
	count := 0.0

	series := metrics.Series{
		Target:     index,
		DataPoints: make([][]float64, 0),
	}
	for _, order := range orders {
		if timeRange.IsWithin(order.Order.Time) {
			count++
			series.DataPoints = append(series.DataPoints, []float64{count, float64(cointime.ToMilli(order.Order.Time))})
		}
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
