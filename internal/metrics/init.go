package metrics

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const port = 6040

func init() {

	p := port + rand.Intn(50)

	Observer = &Metrics{
		mutex:      new(sync.RWMutex),
		prometheus: newPrometheusMetrics(),
	}

	prometheus.MustRegister(Observer.prometheus.Trades)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		fmt.Printf(fmt.Sprintf("Starting metrics server at port %d\n", p))
		err := http.ListenAndServe(fmt.Sprintf(":%d", p), nil)
		if err != nil {
			log.Error().Err(err).Msg("could not start metrics server")
		}
	}()
}
