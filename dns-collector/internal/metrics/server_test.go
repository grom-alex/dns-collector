package metrics

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"dns-collector/internal/config"
)

func TestMetricsServerStartStop(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    9091, // Use different port to avoid conflicts
		Path:    "/metrics",
	}

	registry := NewRegistry()
	server := NewServer(cfg, registry)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop server
	err = server.Stop()
	if err != nil {
		t.Errorf("Failed to stop metrics server: %v", err)
	}
}

func TestMetricsEndpointReturnsPrometheusFormat(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    9092,
		Path:    "/metrics",
	}

	registry := NewRegistry()

	// Add some test metrics
	registry.ServerNewDomains.Inc()
	registry.DBDomainsTotal.Set(100)
	registry.ResolverActiveWorkers.Set(5)
	registry.ResolverLookups.WithLabelValues("ipv4", "success").Add(10)

	server := NewServer(cfg, registry)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			t.Logf("Failed to stop server: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Make request to metrics endpoint
	resp, err := http.Get("http://localhost:9092/metrics")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify Prometheus format
	if !strings.Contains(bodyStr, "# HELP") {
		t.Error("Response missing # HELP lines")
	}

	if !strings.Contains(bodyStr, "# TYPE") {
		t.Error("Response missing # TYPE lines")
	}

	// Verify our custom metrics are present
	expectedMetrics := []string{
		"dns_server_new_domains_total",
		"dns_db_domains_total",
		"dns_resolver_active_workers",
		"dns_resolver_lookups_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("Expected metric %s not found in response", metric)
		}
	}

	// Verify Go runtime metrics are present
	runtimeMetrics := []string{
		"go_goroutines",
		"go_memstats_alloc_bytes",
		"process_cpu_seconds_total",
	}

	for _, metric := range runtimeMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("Expected runtime metric %s not found in response", metric)
		}
	}
}

func TestMetricsEndpointWithLabels(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    9093,
		Path:    "/metrics",
	}

	registry := NewRegistry()

	// Add metrics with labels
	registry.ResolverLookups.WithLabelValues("ipv4", "success").Add(100)
	registry.ResolverLookups.WithLabelValues("ipv6", "success").Add(50)
	registry.ResolverLookups.WithLabelValues("ipv4", "error").Add(5)
	registry.ServerMessagesReceived.WithLabelValues("valid").Add(1000)
	registry.ServerMessagesReceived.WithLabelValues("invalid").Add(10)

	server := NewServer(cfg, registry)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			t.Logf("Failed to stop server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:9093/metrics")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify labels are present
	expectedLabels := []string{
		`ip_version="ipv4"`,
		`ip_version="ipv6"`,
		`status="success"`,
		`status="error"`,
		`status="valid"`,
		`status="invalid"`,
	}

	for _, label := range expectedLabels {
		if !strings.Contains(bodyStr, label) {
			t.Errorf("Expected label %s not found in response", label)
		}
	}
}

func TestMetricsEndpointContentType(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    9094,
		Path:    "/metrics",
	}

	registry := NewRegistry()
	server := NewServer(cfg, registry)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			t.Logf("Failed to stop server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:9094/metrics")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected Content-Type to contain text/plain, got %s", contentType)
	}
}

func TestMetricsServerInvalidPort(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    99999, // Invalid port
		Path:    "/metrics",
	}

	registry := NewRegistry()
	server := NewServer(cfg, registry)

	err := server.Start()
	if err == nil {
		if err := server.Stop(); err != nil {
			t.Logf("Failed to stop server: %v", err)
		}
		t.Error("Expected error when starting server with invalid port")
	}
}

func TestMetricsServerMultipleRequests(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    9095,
		Path:    "/metrics",
	}

	registry := NewRegistry()
	server := NewServer(cfg, registry)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			t.Logf("Failed to stop server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Make multiple concurrent requests
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			resp, err := http.Get("http://localhost:9095/metrics")
			if err != nil {
				results <- err
				return
			}
			defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

			if resp.StatusCode != http.StatusOK {
				results <- http.ErrServerClosed
				return
			}

			results <- nil
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}
}

func TestMetricsServerGracefulShutdown(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    9096,
		Path:    "/metrics",
	}

	registry := NewRegistry()
	server := NewServer(cfg, registry)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Start a long request
	done := make(chan bool)
	go func() {
		resp, err := http.Get("http://localhost:9096/metrics")
		if err == nil {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}
		done <- true
	}()

	// Wait for request to start
	time.Sleep(50 * time.Millisecond)

	// Stop server (should wait for request to complete)
	err = server.Stop()
	if err != nil {
		t.Errorf("Error during graceful shutdown: %v", err)
	}

	// Wait for request to finish
	<-done
}

func TestMetricsEndpoint404OnWrongPath(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    9097,
		Path:    "/metrics",
	}

	registry := NewRegistry()
	server := NewServer(cfg, registry)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			t.Logf("Failed to stop server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Request wrong path
	resp, err := http.Get("http://localhost:9097/wrong-path")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}
