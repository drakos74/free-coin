package metrics

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const port = 6040

func init() {

	// This is not ideal , but would do for now.
	// We need to differentiate the metrics , because all processes run on the same server.
	rand.Seed(time.Now().UTC().UnixNano())
	p := port

	Observer = &Metrics{
		mutex:      new(sync.RWMutex),
		prometheus: newPrometheusMetrics(),
	}

	prometheus.MustRegister(Observer.prometheus.Trades)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Info().Int("port", p).Msg("Starting metrics server")
		err := http.ListenAndServe(fmt.Sprintf(":%d", p), nil)
		if err != nil {
			log.Error().Err(err).Msg("could not start metrics server")
		}
	}()
}
