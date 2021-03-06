package metrics

import "github.com/prometheus/client_golang/prometheus"

type prometheusMetrics struct {
	Trades *prometheus.CounterVec
}

func newPrometheusMetrics() prometheusMetrics {
	return prometheusMetrics{Trades: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "coin",
			Name:      "trades",
		}, []string{"coin", "process"}),
	}
}
