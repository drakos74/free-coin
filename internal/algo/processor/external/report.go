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

				sum := 0.0

				var sortedOrders Orders
				sortedOrders = orders
				sort.Sort(sortedOrders)

				series := metrics.Series{
					Target:     filepath.Base(dir),
					DataPoints: make([][]float64, 0),
				}
				var close bool
				for _, order := range sortedOrders {
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
