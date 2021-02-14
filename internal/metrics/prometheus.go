package metrics

import "github.com/prometheus/client_golang/prometheus"

type Prometheus struct {
	Trades *prometheus.CounterVec
}

func NewPrometheusMetrics() Prometheus {
	return Prometheus{Trades: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "coin",
			Name:      "trades",
		}, []string{"coin", "process"}),
	}
}
