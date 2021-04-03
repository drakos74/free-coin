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

const (
	intervalKey   = "interval"
	accumulateKey = "accumulate"
)

type queryGenerator struct {
	dir       string
	index     string
	timeRange cointime.Range
	registry  storage.Registry
	data      map[string]interface{}
	prices    map[model.Coin]model.CurrentPrice
	condition func(dir string) bool
	addSeries func(query query) metrics.Series
}

type query struct {
	index     string
	timeRange cointime.Range
	data      map[string]interface{}
	prices    map[model.Coin]model.CurrentPrice
	orders    Orders
}

func addTargets(client api.Exchange, grafana *metrics.Server, registry storage.Registry) {
	registryPath := filepath.Join(storage.DefaultDir, storage.RegistryDir, storage.SignalsPath)
	grafana.Target("PnL", readFromRegistry(client, registryPath, registry, noError, addPnL))
	grafana.Target("trades", readFromRegistry(client, registryPath, registry, noError, count))
	grafana.Target("errors", readFromRegistry(client, registryPath, registry, isError, count))
}

func readFromRegistry(client api.Exchange, registryPath string, registry storage.Registry, condition func(dir string) bool, addSeries func(query query) metrics.Series) metrics.TargetQuery {
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

				qq := queryGenerator{
					dir:       dir,
					index:     index,
					timeRange: timeRange,
					registry:  registry,
					data:      data,
					prices:    prices,
					condition: condition,
					addSeries: addSeries,
				}

				if _, ok := accountFor[index]; ok {
					return nil
				}
				series, err := parseEvents(qq)
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

func parseEvents(queryGen queryGenerator) (metrics.Series, error) {
	if queryGen.condition(queryGen.dir) {

		orders := []Order{{}}
		// look up one level up
		key := storage.K{
			Pair:  filepath.Base(filepath.Dir(queryGen.dir)),
			Label: queryGen.index,
		}
		err := queryGen.registry.GetAll(key, &orders)
		if err != nil {
			return metrics.Series{}, fmt.Errorf("could not read from registry: %w", err)
		}
		var sortedOrders Orders
		sortedOrders = orders
		sort.Sort(sortedOrders)

		qq := query{
			index:     queryGen.index,
			timeRange: queryGen.timeRange,
			data:      queryGen.data,
			prices:    queryGen.prices,
			orders:    sortedOrders,
		}

		return queryGen.addSeries(qq), nil
	}
	return metrics.Series{}, fmt.Errorf("invalid dir")
}

// addPnL calculates the pnl for the given query
// note , the orders must always be sorted within the query
func addPnL(query query) metrics.Series {
	series := metrics.Series{
		Target:     query.index,
		DataPoints: make([][]float64, 0),
	}
	sum := 0.0
	var open bool
	var lastOrder Order
	openingOrders := make(map[string]Order)

	interval, err := parseDuration(intervalKey, query.data)
	if err != nil {
		log.Warn().Err(err).Msg("could not parse query data")
	}
	acc := parseBool(accumulateKey, query.data)

	hash := cointime.NewHash(interval)

	ss := make(map[int64]float64)

	var lh int64
	// we must assume time ordering of the orders for the below logic to work.
	for _, order := range query.orders {
		// NOTE : we want to know of any orders that were closed within the time range
		if !query.timeRange.IsBeforeEnd(order.Order.Time) {
			continue
		}

		h := hash.Do(order.Order.Time) + 1 // +1 because we want to assign the order to the end of the interval

		fmt.Println(fmt.Sprintf("%s = %+v", order.Order.Coin, order.Order))

		if _, ok := ss[h]; !ok {
			if acc {
				// start from previous interval
				ss[h] = ss[lh]
			} else {
				// start anew
				ss[h] = 0.0
			}
		}

		if order.Order.RefID == "" {
			openingOrders[order.Order.ID] = order
			open = true
		} else {
			// else lets find the opening order
			if o, ok := openingOrders[order.Order.RefID]; ok {
				sum += order.Order.Value() + o.Order.Value()
				ss[h] = ss[h] + sum
			} else {
				log.Error().Str("coin", string(order.Order.Coin)).Str("ref-id", order.Order.RefID).Msg("could not pair order")
			}
			open = false
		}
		if h > lh {
			// add the previous one ... if it s not the first one ...
			if lh > 0 {
				v := ss[lh]
				fmt.Println(fmt.Sprintf("v = %+v", v))
				series.DataPoints = append(series.DataPoints, []float64{v, float64(cointime.ToMilli(hash.Undo(lh)))})
			}
			lh = h
		}
		lastOrder = order
	}
	// TODO : how to interpolate ...
	now := time.Now()
	// if we are at the last one .. we ll add a virtual one at the current price
	if open && len(query.orders)%2 != 0 {
		// we only extrapolate if the time range is close to now
		// and the last order was an opening order
		// and we have a current price for the asset
		if p, ok := query.prices[lastOrder.Order.Coin]; ok && math.Abs(now.Sub(query.timeRange.To).Minutes()) < 30 {
			// if we have a last price for this asset ...
			sum += (p.Price - lastOrder.Order.Price) * lastOrder.Order.Volume
			series.DataPoints = append(series.DataPoints, []float64{sum, float64(cointime.ToMilli(query.timeRange.To))})
		}
	}
	return series
}

func count(query query) metrics.Series {
	count := 0.0

	series := metrics.Series{
		Target:     query.index,
		DataPoints: make([][]float64, 0),
	}
	for _, order := range query.orders {
		if query.timeRange.IsWithin(order.Order.Time) {
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

func parseDuration(key string, data map[string]interface{}) (time.Duration, error) {
	if s, ok := data[key]; ok {
		d, err := time.ParseDuration(fmt.Sprintf("%v", s))
		if err != nil {
			return 0, fmt.Errorf("could not pars duration from %v: %w", s, err)
		}
		return d, nil
	}
	return 0, fmt.Errorf("key does not exist: %s", key)
}

func parseBool(key string, data map[string]interface{}) bool {
	if s, ok := data[key]; ok {
		if b, ok := s.(bool); ok {
			return b
		}
	}
	return false
}
