package external

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

func addTargets(grafana *metrics.Server, registry storage.Registry) {
	registryPath := filepath.Join(storage.DefaultDir, storage.RegistryDir, storage.SignalsPath)
	// TODO : get current prices ...
	err := filepath.Walk(registryPath, func(path string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() {
			dir := filepath.Dir(path)
			grafana.Target(dir, func(data map[string]interface{}) metrics.Series {
				orders := []Order{{}}
				key := storage.K{
					Pair:  filepath.Base(filepath.Dir(dir)),
					Label: filepath.Base(dir),
				}
				err := registry.GetAll(key, &orders)
				if err != nil {
					log.Error().Str("key", fmt.Sprintf("%+v", key)).Err(err).Msg("could not parse orders")
				}

				series := metrics.Series{
					Target:     filepath.Base(dir),
					DataPoints: make([][]float64, len(orders)),
				}

				sum := 0.0

				var sortedOrders Orders
				sortedOrders = orders
				sort.Sort(sortedOrders)
				fmt.Println(fmt.Sprintf("sortedOrders = %+v", sortedOrders))
				if len(sortedOrders)%2 == 1 {
					// remove the last ...
					sortedOrders = sortedOrders[:len(sortedOrders)-1]
				}
				for i, order := range sortedOrders {
					switch order.Order.Type {
					case model.Buy:
						sum -= order.Order.Price * order.Order.Volume
					case model.Sell:
						sum += order.Order.Price * order.Order.Volume
					}
					series.DataPoints[i] = []float64{sum, float64(cointime.ToMilli(order.Order.Time))}
				}
				return series
			})
		}

		return nil

	})
	if err != nil {
		log.Error().
			Str("path", registryPath).
			Err(err).
			Msg("could not look into registry path")
	}
}
