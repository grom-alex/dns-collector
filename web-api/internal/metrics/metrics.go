package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry holds all metrics collectors for the web-api service.
type Registry struct {
	registry *prometheus.Registry

	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge
	HTTPResponseSize     *prometheus.HistogramVec

	// API metrics
	APIStatsQueries   prometheus.Counter
	APIDomainsQueries prometheus.Counter
	APIExportTotal    *prometheus.CounterVec

	// Database metrics
	DBDomainsTotal    prometheus.Gauge
	DBIPsTotal        prometheus.Gauge
	DBConnectionsOpen prometheus.Gauge
	DBConnectionsIdle prometheus.Gauge
}

// NewRegistry creates a new metrics registry with all collectors registered.
func NewRegistry() *Registry {
	reg := prometheus.NewRegistry()

	// Register Go runtime metrics
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	r := &Registry{
		registry: reg,

		// HTTP metrics
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path"},
		),
		HTTPRequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
		),
		HTTPResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "Size of HTTP responses in bytes",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
			},
			[]string{"method", "path"},
		),

		// API metrics
		APIStatsQueries: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "api_stats_queries_total",
				Help: "Total number of queries to /api/stats",
			},
		),
		APIDomainsQueries: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "api_domains_queries_total",
				Help: "Total number of queries to /api/domains",
			},
		),
		APIExportTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_export_generated_total",
				Help: "Total number of exports generated",
			},
			[]string{"type"},
		),

		// Database metrics
		DBDomainsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_domains_total",
				Help: "Total number of domains in the database",
			},
		),
		DBIPsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_ips_total",
				Help: "Total number of IP addresses in the database",
			},
		),
		DBConnectionsOpen: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_open",
				Help: "Number of open database connections",
			},
		),
		DBConnectionsIdle: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_idle",
				Help: "Number of idle database connections",
			},
		),
	}

	// Register all metrics
	reg.MustRegister(
		r.HTTPRequestsTotal,
		r.HTTPRequestDuration,
		r.HTTPRequestsInFlight,
		r.HTTPResponseSize,
		r.APIStatsQueries,
		r.APIDomainsQueries,
		r.APIExportTotal,
		r.DBDomainsTotal,
		r.DBIPsTotal,
		r.DBConnectionsOpen,
		r.DBConnectionsIdle,
	)

	return r
}

// GetRegistry returns the underlying Prometheus registry.
func (r *Registry) GetRegistry() *prometheus.Registry {
	return r.registry
}
