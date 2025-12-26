package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if r.registry == nil {
		t.Error("Registry.registry is nil")
	}

	// Test that all metrics are initialized
	if r.ResolverDomainsProcessed == nil {
		t.Error("ResolverDomainsProcessed is nil")
	}
	if r.ResolverLookups == nil {
		t.Error("ResolverLookups is nil")
	}
	if r.ResolverLookupDuration == nil {
		t.Error("ResolverLookupDuration is nil")
	}
	if r.ResolverBatchSize == nil {
		t.Error("ResolverBatchSize is nil")
	}
	if r.ResolverActiveWorkers == nil {
		t.Error("ResolverActiveWorkers is nil")
	}
	if r.ServerMessagesReceived == nil {
		t.Error("ServerMessagesReceived is nil")
	}
	if r.ServerDomainsReceived == nil {
		t.Error("ServerDomainsReceived is nil")
	}
	if r.ServerNewDomains == nil {
		t.Error("ServerNewDomains is nil")
	}
	if r.ServerProcessingTime == nil {
		t.Error("ServerProcessingTime is nil")
	}
	if r.CleanupStatsDeleted == nil {
		t.Error("CleanupStatsDeleted is nil")
	}
	if r.CleanupIPsDeleted == nil {
		t.Error("CleanupIPsDeleted is nil")
	}
	if r.CleanupDuration == nil {
		t.Error("CleanupDuration is nil")
	}
	if r.CleanupRuns == nil {
		t.Error("CleanupRuns is nil")
	}
	if r.DBDomainsTotal == nil {
		t.Error("DBDomainsTotal is nil")
	}
	if r.DBIPsTotal == nil {
		t.Error("DBIPsTotal is nil")
	}
	if r.DBConnectionsOpen == nil {
		t.Error("DBConnectionsOpen is nil")
	}
	if r.DBConnectionsIdle == nil {
		t.Error("DBConnectionsIdle is nil")
	}
	if r.DBQueriesTotal == nil {
		t.Error("DBQueriesTotal is nil")
	}
	if r.DBQueryDuration == nil {
		t.Error("DBQueryDuration is nil")
	}
}

func TestRegistryMetricsCanBeUsed(t *testing.T) {
	r := NewRegistry()

	// Test counter operations
	r.ResolverDomainsProcessed.WithLabelValues("success").Inc()
	r.ResolverDomainsProcessed.WithLabelValues("error").Add(5)

	r.ResolverLookups.WithLabelValues("ipv4", "success").Inc()
	r.ResolverLookups.WithLabelValues("ipv6", "error").Inc()

	r.ServerMessagesReceived.WithLabelValues("valid").Inc()
	r.ServerDomainsReceived.WithLabelValues("dns").Inc()
	r.ServerNewDomains.Inc()

	r.CleanupStatsDeleted.Add(100)
	r.CleanupIPsDeleted.Add(50)
	r.CleanupRuns.Inc()

	r.DBQueriesTotal.WithLabelValues("select", "success").Inc()

	// Test gauge operations
	r.ResolverBatchSize.Set(10)
	r.ResolverActiveWorkers.Set(5)
	r.DBDomainsTotal.Set(1000)
	r.DBIPsTotal.Set(5000)
	r.DBConnectionsOpen.Set(10)
	r.DBConnectionsIdle.Set(3)

	// Test histogram operations
	r.ResolverLookupDuration.WithLabelValues("ipv4").Observe(0.05)
	r.ServerProcessingTime.Observe(0.001)
	r.CleanupDuration.Observe(5.0)
	r.DBQueryDuration.WithLabelValues("select").Observe(0.01)

	// Verify metrics can be gathered
	mfs, err := r.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(mfs) == 0 {
		t.Error("No metrics gathered")
	}

	// Check that our custom metrics are present
	metricNames := make(map[string]bool)
	for _, mf := range mfs {
		metricNames[mf.GetName()] = true
	}

	expectedMetrics := []string{
		"dns_resolver_domains_processed_total",
		"dns_resolver_lookups_total",
		"dns_resolver_lookup_duration_seconds",
		"dns_resolver_batch_size",
		"dns_resolver_active_workers",
		"dns_server_messages_received_total",
		"dns_server_domains_received_total",
		"dns_server_new_domains_total",
		"dns_server_processing_duration_seconds",
		"dns_cleanup_stats_deleted_total",
		"dns_cleanup_ips_deleted_total",
		"dns_cleanup_duration_seconds",
		"dns_cleanup_runs_total",
		"dns_db_domains_total",
		"dns_db_ips_total",
		"dns_db_connections_open",
		"dns_db_connections_idle",
		"dns_db_queries_total",
		"dns_db_query_duration_seconds",
	}

	for _, name := range expectedMetrics {
		if !metricNames[name] {
			t.Errorf("Expected metric %s not found", name)
		}
	}
}

func TestGetRegistry(t *testing.T) {
	r := NewRegistry()

	reg := r.GetRegistry()
	if reg == nil {
		t.Error("GetRegistry returned nil")
	}

	// Verify it's a valid Prometheus registry
	_, ok := interface{}(reg).(*prometheus.Registry)
	if !ok {
		t.Error("GetRegistry did not return a *prometheus.Registry")
	}
}
