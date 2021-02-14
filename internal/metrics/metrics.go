package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var Observer = &Metrics{
	mutex:      new(sync.RWMutex),
	prometheus: NewPrometheusMetrics(),
}

func init() {
	prometheus.MustRegister(Observer.prometheus.Trades)
}

type Metrics struct {
	mutex      *sync.RWMutex
	prometheus Prometheus
}

func (m *Metrics) Increment(labels ...string) {
	m.prometheus.Trades.WithLabelValues(labels...).Inc()
}
