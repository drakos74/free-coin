package metrics

import "github.com/prometheus/client_golang/prometheus"

type prometheusMetrics struct {
	Events   *prometheus.CounterVec
	Trades   *prometheus.CounterVec
	Lag      *prometheus.GaugeVec
	Duration *prometheus.HistogramVec
	Calls    *prometheus.CounterVec
	Errors   *prometheus.CounterVec
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
			}, []string{"coin", "process", "step"}),
		Duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "coin",
				Name:      "duration",
				Buckets:   []float64{.05, .1, .5, 1, 5, 10, 15, 30, 60, 120},
			}, []string{"coin", "process", "routine"}),
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
