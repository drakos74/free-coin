package metrics

import (
	"sync"
)

var Observer *Metrics

type Metrics struct {
	mutex      *sync.RWMutex
	prometheus prometheusMetrics
}

func (m *Metrics) IncrementTrades(labels ...string) {
	m.prometheus.Trades.WithLabelValues(labels...).Inc()
}
