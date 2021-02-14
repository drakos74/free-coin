package metrics

import (
	"sync"
)

var Observer = &Metrics{
	mutex:      new(sync.RWMutex),
	prometheus: newPrometheusMetrics(),
}

type Metrics struct {
	mutex      *sync.RWMutex
	prometheus prometheusMetrics
}

func (m *Metrics) IncrementTrades(labels ...string) {
	m.prometheus.Trades.WithLabelValues(labels...).Inc()
}
