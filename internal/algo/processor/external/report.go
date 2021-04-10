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

func addTargets(prefix string, client api.Exchange, grafana *metrics.Server, registry storage.Registry) {
	registryPath := filepath.Join(storage.DefaultDir, storage.RegistryDir, storage.SignalsPath)
	grafana.Target(fmt.Sprintf("%s-%s", prefix, "PnL"), readFromRegistry(client, registryPath, registry, noError, addPnL))
	grafana.Target(fmt.Sprintf("%s-%s", prefix, "trades"), readFromRegistry(client, registryPath, registry, noError, count))
	grafana.Target(fmt.Sprintf("%s-%s", prefix, "errors"), readFromRegistry(client, registryPath, registry, isError, count))
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

	openingOrders := make(map[string]Order)

	qd := ParseQueryData(query.data)

	series := metrics.Series{
		Target:     fmt.Sprintf("%s[%v]", query.index, qd),
		DataPoints: make([][]float64, 0),
	}

	hash := cointime.NewHash(qd.Interval)

	ss := make(map[int64]float64)

	var lastValue float64
	var lastOrder Order

	var lh int64
	// we must assume time ordering of the orders for the below logic to work.
	for _, order := range query.orders {
		// NOTE : we want to know of any orders that were closed within the time range
		if !query.timeRange.IsBeforeEnd(order.Order.Time) {
			continue
		}

		h := hash.Do(order.Order.Time) + 1 // +1 because we want to assign the order to the end of the interval

		if _, ok := ss[h]; !ok {
			if qd.Acc {
				// start from previous interval
				ss[h] = ss[lh]
			} else {
				// start anew
				ss[h] = 0.0
			}
		}

		if order.Order.RefID == "" {
			openingOrders[order.Order.ID] = order
		} else {
			// else lets find the opening order
			if o, ok := openingOrders[order.Order.RefID]; ok {
				net := order.Order.Value() + o.Order.Value()
				ss[h] = ss[h] + net
			} else {
				log.Error().Str("coin", string(order.Order.Coin)).Str("ref-account", order.Order.RefID).Msg("could not pair order")
			}
		}
		if h > lh {
			// add the previous one ... if it s not the first one ...
			if lh > 0 {
				v := ss[lh]
				lastValue = v
				series.DataPoints = append(series.DataPoints, []float64{v, float64(cointime.ToMilli(hash.Undo(lh)))})
			}
			lh = h
		}
		lastOrder = order
	}

	last := hash.Undo(lh)
	now := time.Now()

	if last.Before(now) {
		lastValue += ss[lh]
		series.DataPoints = append(series.DataPoints, []float64{lastValue, float64(cointime.ToMilli(last))})
	}

	// add a virtual trade for now ... if the last one is an open one
	p, ok := query.prices[lastOrder.Order.Coin]
	lastIsOpen := lastOrder.Order.RefID == ""
	EndTimeIsNow := math.Abs(now.Sub(query.timeRange.To).Minutes())
	if ok &&
		lastIsOpen &&
		EndTimeIsNow < 1 {
		// if we have a last price for this asset ...
		// and we re left with an open order
		if lastOrder.Order.Type == model.Buy {
			lastValue += (p.Price - lastOrder.Order.Price) * lastOrder.Order.Volume
		} else {
			lastValue += (lastOrder.Order.Price - p.Price) * lastOrder.Order.Volume
		}
		series.DataPoints = append(series.DataPoints, []float64{lastValue, float64(cointime.ToMilli(query.timeRange.To))})
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
