package metrics

import "github.com/prometheus/client_golang/prometheus"

type prometheusMetrics struct {
	Events *prometheus.CounterVec
	Trades *prometheus.CounterVec
	Lag    *prometheus.GaugeVec
	Calls  *prometheus.CounterVec
	Errors *prometheus.CounterVec
}

func newPrometheusMetrics() prometheusMetrics {
	return prometheusMetrics{
		Events: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "coin",
				Name:      "events",
			}, []string{"coin", "duration", "action", "process"},
		),
		Trades: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "coin",
				Name:      "trades",
			}, []string{"coin", "process"},
		),
		Lag: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "coin",
				Name:      "lag",
			}, []string{"coin", "process"}),
		Calls: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "call",
				Name:      "processor",
			}, []string{"call", "process"},
		),
		Errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "error",
				Name:      "processor",
			}, []string{"coin", "process"},
		),
	}
}
