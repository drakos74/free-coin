package metrics

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const port = 6021

func init() {

	Observer = &Metrics{
		mutex:      new(sync.RWMutex),
		prometheus: newPrometheusMetrics(),
	}

	prometheus.MustRegister(Observer.prometheus.Trades)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			log.Error().Err(err).Msg("could not start metrics server")
		}
	}()
}
