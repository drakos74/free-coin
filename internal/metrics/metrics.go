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

func (m *Metrics) IncrementCalls(labels ...string) {
	m.prometheus.Calls.WithLabelValues(labels...).Inc()
}
