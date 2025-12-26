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
	if r.HTTPRequestsTotal == nil {
		t.Error("HTTPRequestsTotal is nil")
	}
	if r.HTTPRequestDuration == nil {
		t.Error("HTTPRequestDuration is nil")
	}
	if r.HTTPRequestsInFlight == nil {
		t.Error("HTTPRequestsInFlight is nil")
	}
	if r.HTTPResponseSize == nil {
		t.Error("HTTPResponseSize is nil")
	}
	if r.APIStatsQueries == nil {
		t.Error("APIStatsQueries is nil")
	}
	if r.APIDomainsQueries == nil {
		t.Error("APIDomainsQueries is nil")
	}
	if r.APIExportTotal == nil {
		t.Error("APIExportTotal is nil")
	}
}

func TestRegistryMetricsCanBeUsed(t *testing.T) {
	r := NewRegistry()

	// Test counter operations
	r.HTTPRequestsTotal.WithLabelValues("GET", "/api/stats", "200").Inc()
	r.HTTPRequestsTotal.WithLabelValues("POST", "/api/domains", "201").Inc()

	r.APIStatsQueries.Inc()
	r.APIDomainsQueries.Inc()
	r.APIExportTotal.WithLabelValues("stats_excel").Inc()
	r.APIExportTotal.WithLabelValues("domains_excel").Inc()

	// Test gauge operations
	r.HTTPRequestsInFlight.Set(5)

	// Test histogram operations
	r.HTTPRequestDuration.WithLabelValues("GET", "/api/stats").Observe(0.05)
	r.HTTPResponseSize.WithLabelValues("GET", "/api/stats").Observe(1024)

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
		"http_requests_total",
		"http_request_duration_seconds",
		"http_requests_in_flight",
		"http_response_size_bytes",
		"api_stats_queries_total",
		"api_domains_queries_total",
		"api_export_generated_total",
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
