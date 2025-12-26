package metrics

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMetricsHandlerReturnsPrometheusFormat(t *testing.T) {
	registry := NewRegistry()

	// Add some test metrics
	registry.APIStatsQueries.Inc()
	registry.APIDomainsQueries.Add(5)
	registry.HTTPRequestsInFlight.Set(3)
	registry.HTTPRequestsTotal.WithLabelValues("GET", "/api/stats", "200").Inc()

	handler := NewMetricsHandler(registry)

	// Create test server
	server := &http.Server{
		Addr:    ":9098",
		Handler: handler,
	}

	go func() {
		server.ListenAndServe()
	}()
	defer server.Close()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Make request to metrics endpoint
	resp, err := http.Get("http://localhost:9098/")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

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
		"api_stats_queries_total",
		"api_domains_queries_total",
		"http_requests_in_flight",
		"http_requests_total",
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

func TestMetricsHandlerWithHTTPLabels(t *testing.T) {
	registry := NewRegistry()

	// Add HTTP metrics with various labels
	registry.HTTPRequestsTotal.WithLabelValues("GET", "/api/stats", "200").Add(100)
	registry.HTTPRequestsTotal.WithLabelValues("GET", "/api/domains", "200").Add(50)
	registry.HTTPRequestsTotal.WithLabelValues("POST", "/api/stats", "400").Add(5)
	registry.HTTPRequestsTotal.WithLabelValues("GET", "/api/stats", "500").Add(2)

	registry.HTTPRequestDuration.WithLabelValues("GET", "/api/stats").Observe(0.1)
	registry.HTTPResponseSize.WithLabelValues("GET", "/api/domains").Observe(1024)

	handler := NewMetricsHandler(registry)

	server := &http.Server{
		Addr:    ":9099",
		Handler: handler,
	}

	go func() {
		server.ListenAndServe()
	}()
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:9099/")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify HTTP labels are present
	expectedLabels := []string{
		`method="GET"`,
		`method="POST"`,
		`path="/api/stats"`,
		`path="/api/domains"`,
		`status="200"`,
		`status="400"`,
		`status="500"`,
	}

	for _, label := range expectedLabels {
		if !strings.Contains(bodyStr, label) {
			t.Errorf("Expected label %s not found in response", label)
		}
	}
}

func TestMetricsHandlerContentType(t *testing.T) {
	registry := NewRegistry()
	handler := NewMetricsHandler(registry)

	server := &http.Server{
		Addr:    ":9100",
		Handler: handler,
	}

	go func() {
		server.ListenAndServe()
	}()
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:9100/")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected Content-Type to contain text/plain, got %s", contentType)
	}
}

func TestMetricsHandlerMultipleRequests(t *testing.T) {
	registry := NewRegistry()
	handler := NewMetricsHandler(registry)

	server := &http.Server{
		Addr:    ":9101",
		Handler: handler,
	}

	go func() {
		server.ListenAndServe()
	}()
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	// Make multiple concurrent requests
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			resp, err := http.Get("http://localhost:9101/")
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()

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

func TestMetricsHandlerExportTypes(t *testing.T) {
	registry := NewRegistry()

	// Add export metrics
	registry.APIExportTotal.WithLabelValues("stats_excel").Add(10)
	registry.APIExportTotal.WithLabelValues("domains_excel").Add(5)

	handler := NewMetricsHandler(registry)

	server := &http.Server{
		Addr:    ":9102",
		Handler: handler,
	}

	go func() {
		server.ListenAndServe()
	}()
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:9102/")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify export type labels
	expectedLabels := []string{
		`type="stats_excel"`,
		`type="domains_excel"`,
	}

	for _, label := range expectedLabels {
		if !strings.Contains(bodyStr, label) {
			t.Errorf("Expected label %s not found in response", label)
		}
	}

	// Verify metric name
	if !strings.Contains(bodyStr, "api_export_generated_total") {
		t.Error("Expected metric api_export_generated_total not found")
	}
}

func TestMetricsHandlerHistogramBuckets(t *testing.T) {
	registry := NewRegistry()

	// Add histogram observations
	for i := 0; i < 100; i++ {
		registry.HTTPRequestDuration.WithLabelValues("GET", "/api/stats").Observe(0.05)
	}
	for i := 0; i < 10; i++ {
		registry.HTTPRequestDuration.WithLabelValues("GET", "/api/stats").Observe(0.5)
	}
	for i := 0; i < 5; i++ {
		registry.HTTPRequestDuration.WithLabelValues("GET", "/api/stats").Observe(2.0)
	}

	handler := NewMetricsHandler(registry)

	server := &http.Server{
		Addr:    ":9103",
		Handler: handler,
	}

	go func() {
		server.ListenAndServe()
	}()
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:9103/")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify histogram buckets are present
	if !strings.Contains(bodyStr, "_bucket{") {
		t.Error("Expected histogram buckets not found")
	}

	if !strings.Contains(bodyStr, "_sum") {
		t.Error("Expected histogram sum not found")
	}

	if !strings.Contains(bodyStr, "_count") {
		t.Error("Expected histogram count not found")
	}

	// Verify le label for buckets
	if !strings.Contains(bodyStr, `le="`) {
		t.Error("Expected le label for histogram buckets not found")
	}
}

func TestNewMetricsHandler(t *testing.T) {
	registry := NewRegistry()
	handler := NewMetricsHandler(registry)

	if handler == nil {
		t.Fatal("NewMetricsHandler returned nil")
	}
}
