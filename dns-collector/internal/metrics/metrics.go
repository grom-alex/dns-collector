package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry holds all metrics collectors for the dns-collector service.
type Registry struct {
	registry *prometheus.Registry

	// Resolver metrics
	ResolverDomainsProcessed *prometheus.CounterVec
	ResolverLookups          *prometheus.CounterVec
	ResolverLookupDuration   *prometheus.HistogramVec
	ResolverBatchSize        prometheus.Gauge
	ResolverActiveWorkers    prometheus.Gauge

	// UDP Server metrics
	ServerMessagesReceived *prometheus.CounterVec
	ServerDomainsReceived  *prometheus.CounterVec
	ServerNewDomains       prometheus.Counter
	ServerProcessingTime   prometheus.Histogram

	// Cleanup metrics
	CleanupStatsDeleted prometheus.Counter
	CleanupIPsDeleted   prometheus.Counter
	CleanupDuration     prometheus.Histogram
	CleanupRuns         prometheus.Counter

	// Database metrics
	DBDomainsTotal     prometheus.Gauge
	DBIPsTotal         prometheus.Gauge
	DBConnectionsOpen  prometheus.Gauge
	DBConnectionsIdle  prometheus.Gauge
	DBQueriesTotal     *prometheus.CounterVec
	DBQueryDuration    *prometheus.HistogramVec
}

// NewRegistry creates a new metrics registry with all collectors registered.
func NewRegistry() *Registry {
	reg := prometheus.NewRegistry()

	// Register Go runtime metrics
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	r := &Registry{
		registry: reg,

		// Resolver metrics
		ResolverDomainsProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dns_resolver_domains_processed_total",
				Help: "Total number of domains processed by the resolver",
			},
			[]string{"status"},
		),
		ResolverLookups: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dns_resolver_lookups_total",
				Help: "Total number of DNS lookups performed",
			},
			[]string{"ip_version", "status"},
		),
		ResolverLookupDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dns_resolver_lookup_duration_seconds",
				Help:    "Duration of DNS lookup operations",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"ip_version"},
		),
		ResolverBatchSize: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "dns_resolver_batch_size",
				Help: "Number of domains in the current resolution batch",
			},
		),
		ResolverActiveWorkers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "dns_resolver_active_workers",
				Help: "Number of currently active resolver workers",
			},
		),

		// UDP Server metrics
		ServerMessagesReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dns_server_messages_received_total",
				Help: "Total number of UDP messages received",
			},
			[]string{"status"},
		),
		ServerDomainsReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dns_server_domains_received_total",
				Help: "Total number of domains received via UDP",
			},
			[]string{"rtype"},
		),
		ServerNewDomains: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_server_new_domains_total",
				Help: "Total number of new unique domains registered",
			},
		),
		ServerProcessingTime: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "dns_server_processing_duration_seconds",
				Help:    "Time spent processing UDP messages",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
			},
		),

		// Cleanup metrics
		CleanupStatsDeleted: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_cleanup_stats_deleted_total",
				Help: "Total number of old stats records deleted",
			},
		),
		CleanupIPsDeleted: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_cleanup_ips_deleted_total",
				Help: "Total number of expired IP addresses deleted",
			},
		),
		CleanupDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "dns_cleanup_duration_seconds",
				Help:    "Duration of cleanup operations",
				Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60},
			},
		),
		CleanupRuns: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_cleanup_runs_total",
				Help: "Total number of cleanup runs",
			},
		),

		// Database metrics
		DBDomainsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "dns_db_domains_total",
				Help: "Total number of domains in the database",
			},
		),
		DBIPsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "dns_db_ips_total",
				Help: "Total number of IP addresses in the database",
			},
		),
		DBConnectionsOpen: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "dns_db_connections_open",
				Help: "Number of open database connections",
			},
		),
		DBConnectionsIdle: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "dns_db_connections_idle",
				Help: "Number of idle database connections",
			},
		),
		DBQueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dns_db_queries_total",
				Help: "Total number of database queries",
			},
			[]string{"operation", "status"},
		),
		DBQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dns_db_query_duration_seconds",
				Help:    "Duration of database queries",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
			},
			[]string{"operation"},
		),
	}

	// Register all metrics
	reg.MustRegister(
		r.ResolverDomainsProcessed,
		r.ResolverLookups,
		r.ResolverLookupDuration,
		r.ResolverBatchSize,
		r.ResolverActiveWorkers,
		r.ServerMessagesReceived,
		r.ServerDomainsReceived,
		r.ServerNewDomains,
		r.ServerProcessingTime,
		r.CleanupStatsDeleted,
		r.CleanupIPsDeleted,
		r.CleanupDuration,
		r.CleanupRuns,
		r.DBDomainsTotal,
		r.DBIPsTotal,
		r.DBConnectionsOpen,
		r.DBConnectionsIdle,
		r.DBQueriesTotal,
		r.DBQueryDuration,
	)

	return r
}

// GetRegistry returns the underlying Prometheus registry.
func (r *Registry) GetRegistry() *prometheus.Registry {
	return r.registry
}
