package metrics

import (
	"sync"
)

var Observer *Metrics

type Metrics struct {
	mutex      *sync.RWMutex
	prometheus prometheusMetrics
}

func (m *Metrics) IncrementEvents(labels ...string) {
	m.prometheus.Events.WithLabelValues(labels...).Inc()
}

func (m *Metrics) TrackDuration(f float64, labels ...string) {
	m.prometheus.Duration.WithLabelValues(labels...).Observe(f)
}

func (m *Metrics) IncrementTrades(labels ...string) {
	m.prometheus.Trades.WithLabelValues(labels...).Inc()
}

func (m *Metrics) AddTrades(f float64, labels ...string) {
	m.prometheus.Trades.WithLabelValues(labels...).Add(f)
}

func (m *Metrics) NoteLag(f float64, labels ...string) {
	m.prometheus.Lag.WithLabelValues(labels...).Set(f)
}

func (m *Metrics) IncrementErrors(labels ...string) {
	m.prometheus.Errors.WithLabelValues(labels...).Inc()
}

func (m *Metrics) IncrementCalls(labels ...string) {
	m.prometheus.Calls.WithLabelValues(labels...).Inc()
}
