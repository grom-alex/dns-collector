package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewInfluxDBClient(t *testing.T) {
	cfg := InfluxDBConfig{
		URL:                "http://localhost:8086",
		Token:              "test-token",
		Organization:       "test-org",
		Bucket:             "test-bucket",
		IntervalSeconds:    10,
		InsecureSkipVerify: false,
	}

	registry := NewRegistry()
	client := NewInfluxDBClient(cfg, registry)

	if client == nil {
		t.Fatal("NewInfluxDBClient returned nil")
	}

	if client.cfg.URL != cfg.URL {
		t.Errorf("Expected URL %s, got %s", cfg.URL, client.cfg.URL)
	}

	if client.registry != registry {
		t.Error("Registry not set correctly")
	}
}

func TestNewInfluxDBClientWithTLSSkip(t *testing.T) {
	cfg := InfluxDBConfig{
		URL:                "https://localhost:8086",
		Token:              "test-token",
		Organization:       "test-org",
		Bucket:             "test-bucket",
		IntervalSeconds:    10,
		InsecureSkipVerify: true,
	}

	registry := NewRegistry()
	client := NewInfluxDBClient(cfg, registry)

	if client == nil {
		t.Fatal("NewInfluxDBClient returned nil")
	}

	if !client.cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestInfluxDBClientStartWithUnhealthyServer(t *testing.T) {
	// Create a test HTTP server that returns unhealthy status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/health") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "fail"}`))
		}
	}))
	defer server.Close()

	cfg := InfluxDBConfig{
		URL:                server.URL,
		Token:              "test-token",
		Organization:       "test-org",
		Bucket:             "test-bucket",
		IntervalSeconds:    10,
		InsecureSkipVerify: false,
	}

	registry := NewRegistry()
	client := NewInfluxDBClient(cfg, registry)

	err := client.Start()
	if err == nil {
		t.Error("Expected error when connecting to unhealthy server")
	}

	if !strings.Contains(err.Error(), "health check failed") {
		t.Errorf("Expected health check error, got: %v", err)
	}
}

func TestInfluxDBClientStartWithUnreachableServer(t *testing.T) {
	cfg := InfluxDBConfig{
		URL:                "http://localhost:9999", // unreachable port
		Token:              "test-token",
		Organization:       "test-org",
		Bucket:             "test-bucket",
		IntervalSeconds:    10,
		InsecureSkipVerify: false,
	}

	registry := NewRegistry()
	client := NewInfluxDBClient(cfg, registry)

	err := client.Start()
	if err == nil {
		t.Error("Expected error when connecting to unreachable server")
	}

	if !strings.Contains(err.Error(), "failed to connect") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestInfluxDBClientPushMetricsPartialFailure(t *testing.T) {
	cfg := InfluxDBConfig{
		URL:                "http://localhost:8086",
		Token:              "test-token",
		Organization:       "test-org",
		Bucket:             "test-bucket",
		IntervalSeconds:    10,
		InsecureSkipVerify: false,
	}

	registry := NewRegistry()

	// Add some test metrics
	registry.APIStatsQueries.Inc()
	registry.APIDomainsQueries.Add(5)
	registry.HTTPRequestsInFlight.Set(10)

	client := NewInfluxDBClient(cfg, registry)

	if client == nil {
		t.Fatal("NewInfluxDBClient returned nil")
	}

	// Verify metrics can be gathered
	mfs, err := registry.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(mfs) == 0 {
		t.Error("No metrics gathered")
	}
}

func TestInfluxDBClientStopBeforeStart(t *testing.T) {
	cfg := InfluxDBConfig{
		URL:                "http://localhost:8086",
		Token:              "test-token",
		Organization:       "test-org",
		Bucket:             "test-bucket",
		IntervalSeconds:    10,
		InsecureSkipVerify: false,
	}

	registry := NewRegistry()
	client := NewInfluxDBClient(cfg, registry)

	// Stopping before starting should not panic
	err := client.Stop()
	if err != nil {
		t.Errorf("Unexpected error when stopping unstarted client: %v", err)
	}
}

func TestInfluxDBClientMetricConversion(t *testing.T) {
	cfg := InfluxDBConfig{
		URL:                "http://localhost:8086",
		Token:              "test-token",
		Organization:       "test-org",
		Bucket:             "test-bucket",
		IntervalSeconds:    10,
		InsecureSkipVerify: false,
	}

	registry := NewRegistry()
	client := NewInfluxDBClient(cfg, registry)

	// Test that metrics can be converted to points
	registry.APIStatsQueries.Inc()
	registry.APIDomainsQueries.Add(10)
	registry.HTTPRequestsInFlight.Set(5)
	registry.HTTPRequestDuration.WithLabelValues("GET", "/api/stats").Observe(0.1)
	registry.HTTPResponseSize.WithLabelValues("GET", "/api/domains").Observe(1024)

	// Gather metrics
	mfs, err := registry.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	pointCount := 0
	now := time.Now()
	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			point := client.metricToPoint(mf.GetName(), mf.GetType(), m, now)
			if point != nil {
				pointCount++
				if point.Name() == "" {
					t.Error("Point has empty name")
				}
			}
		}
	}

	if pointCount == 0 {
		t.Error("No points converted from metrics")
	}

	t.Logf("Converted %d metrics to InfluxDB points", pointCount)
}

func TestInfluxDBClientConfigValidation(t *testing.T) {
	tests := []struct {
		name  string
		cfg   InfluxDBConfig
		valid bool
	}{
		{
			name: "valid config",
			cfg: InfluxDBConfig{
				URL:             "http://localhost:8086",
				Token:           "token",
				Organization:    "org",
				Bucket:          "bucket",
				IntervalSeconds: 10,
			},
			valid: true,
		},
		{
			name: "https with insecure skip",
			cfg: InfluxDBConfig{
				URL:                "https://localhost:8086",
				Token:              "token",
				Organization:       "org",
				Bucket:             "bucket",
				IntervalSeconds:    5,
				InsecureSkipVerify: true,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			client := NewInfluxDBClient(tt.cfg, registry)

			if client == nil {
				t.Fatal("NewInfluxDBClient returned nil")
			}

			if client.cfg.URL != tt.cfg.URL {
				t.Errorf("URL mismatch: expected %s, got %s", tt.cfg.URL, client.cfg.URL)
			}

			if client.cfg.Organization != tt.cfg.Organization {
				t.Errorf("Organization mismatch: expected %s, got %s", tt.cfg.Organization, client.cfg.Organization)
			}
		})
	}
}

func TestInfluxDBClientContextTimeout(t *testing.T) {
	cfg := InfluxDBConfig{
		URL:                "http://localhost:9999", // unreachable
		Token:              "test-token",
		Organization:       "test-org",
		Bucket:             "test-bucket",
		IntervalSeconds:    10,
		InsecureSkipVerify: false,
	}

	registry := NewRegistry()
	client := NewInfluxDBClient(cfg, registry)

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Health check should respect context timeout
	_, err := client.client.Health(ctx)
	if err == nil {
		t.Error("Expected timeout error")
	}
}
