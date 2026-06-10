package ws

import "github.com/prometheus/client_golang/prometheus"

var (
	ConnectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ws_connections_active",
		Help: "Number of active WebSocket connections",
	})
	MessagesSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ws_messages_sent_total",
		Help: "Total WebSocket messages sent to clients",
	})
	FanoutLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ws_fanout_latency_seconds",
		Help:    "Time from Redis receive to client send",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	})
)

func init() {
	prometheus.MustRegister(ConnectionsActive, MessagesSent, FanoutLatency)
}
