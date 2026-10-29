// Reference copy. In pprof-demo this lives at internal/metrics/metrics.go.
//
// Package metrics exposes Prometheus metrics for the Gin API.
//
// It uses its own registry (not the global default) so the exported series are
// exactly what is listed here: HTTP RED metrics plus Go runtime and process
// collectors, which give the goroutine / heap graphs that pair with pprof.
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsPath = "/metrics"

type Metrics struct {
	reg      *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inflight prometheus.Gauge
}

func New() *Metrics {
	m := &Metrics{
		reg: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pprofdemo",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests by method, route and status code.",
		}, []string{"method", "route", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "pprofdemo",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency by method and route.",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"method", "route"}),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pprofdemo",
			Name:      "http_requests_in_flight",
			Help:      "Requests currently being served.",
		}),
	}
	m.reg.MustRegister(
		m.requests, m.duration, m.inflight,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// Registerer lets other packages add their own metrics to the same /metrics endpoint.
func (m *Metrics) Registerer() prometheus.Registerer { return m.reg }

// Middleware records request count, latency and in-flight requests.
// It labels by the route template (/api/cpu), never the raw URL, to keep
// label cardinality bounded.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Don't count Prometheus's own scrapes as application traffic.
		if c.FullPath() == metricsPath {
			c.Next()
			return
		}
		start := time.Now()
		m.inflight.Inc()
		defer m.inflight.Dec()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		m.requests.WithLabelValues(c.Request.Method, route, strconv.Itoa(c.Writer.Status())).Inc()
		m.duration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}

// Register mounts GET /metrics on the router.
func (m *Metrics) Register(r *gin.Engine) {
	r.GET(metricsPath, gin.WrapH(promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})))
}
