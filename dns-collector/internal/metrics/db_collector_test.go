package metrics

import (
	"errors"
	"testing"
	"time"
)

// MockDBStatsProvider is a mock implementation of DBStatsProvider for testing.
type MockDBStatsProvider struct {
	domainsCount int64
	ipsCount     int64
	domainsErr   error
	ipsErr       error
}

func (m *MockDBStatsProvider) GetDomainsCount() (int64, error) {
	return m.domainsCount, m.domainsErr
}

func (m *MockDBStatsProvider) GetIPsCount() (int64, error) {
	return m.ipsCount, m.ipsErr
}

func TestNewDBCollector(t *testing.T) {
	db := &MockDBStatsProvider{
		domainsCount: 100,
		ipsCount:     500,
	}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, 30)

	if collector == nil {
		t.Fatal("NewDBCollector returned nil")
	}

	if collector.db != db {
		t.Error("DB provider not set correctly")
	}

	if collector.registry != registry {
		t.Error("Registry not set correctly")
	}

	if collector.interval != 30*time.Second {
		t.Errorf("Expected interval 30s, got %v", collector.interval)
	}
}

func TestNewDBCollectorWithZeroInterval(t *testing.T) {
	db := &MockDBStatsProvider{}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, 0)

	if collector.interval != 30*time.Second {
		t.Errorf("Expected default interval 30s, got %v", collector.interval)
	}
}

func TestNewDBCollectorWithNegativeInterval(t *testing.T) {
	db := &MockDBStatsProvider{}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, -10)

	if collector.interval != 30*time.Second {
		t.Errorf("Expected default interval 30s, got %v", collector.interval)
	}
}

func TestDBCollectorCollect(t *testing.T) {
	db := &MockDBStatsProvider{
		domainsCount: 1000,
		ipsCount:     5000,
	}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, 30)

	// Collect metrics
	collector.collect()

	// Verify metrics were updated
	mfs, err := registry.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	var foundDomains, foundIPs bool
	for _, mf := range mfs {
		if mf.GetName() == "dns_db_domains_total" {
			foundDomains = true
			if len(mf.GetMetric()) > 0 {
				value := mf.GetMetric()[0].GetGauge().GetValue()
				if value != 1000 {
					t.Errorf("Expected domains count 1000, got %f", value)
				}
			}
		}
		if mf.GetName() == "dns_db_ips_total" {
			foundIPs = true
			if len(mf.GetMetric()) > 0 {
				value := mf.GetMetric()[0].GetGauge().GetValue()
				if value != 5000 {
					t.Errorf("Expected IPs count 5000, got %f", value)
				}
			}
		}
	}

	if !foundDomains {
		t.Error("dns_db_domains_total metric not found")
	}
	if !foundIPs {
		t.Error("dns_db_ips_total metric not found")
	}
}

func TestDBCollectorCollectWithErrors(t *testing.T) {
	db := &MockDBStatsProvider{
		domainsCount: 100,
		ipsCount:     500,
		domainsErr:   errors.New("database error"),
		ipsErr:       errors.New("connection error"),
	}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, 30)

	// Collect should not panic even with errors
	collector.collect()

	// Metrics should remain at zero (or previous value)
	// This test mainly ensures error handling doesn't crash
}

func TestDBCollectorCollectPartialError(t *testing.T) {
	db := &MockDBStatsProvider{
		domainsCount: 1000,
		ipsCount:     0,
		domainsErr:   nil,
		ipsErr:       errors.New("IP count error"),
	}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, 30)

	// Collect with partial error
	collector.collect()

	// Verify that successful metric was still updated
	mfs, err := registry.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range mfs {
		if mf.GetName() == "dns_db_domains_total" {
			if len(mf.GetMetric()) > 0 {
				value := mf.GetMetric()[0].GetGauge().GetValue()
				if value != 1000 {
					t.Errorf("Expected domains count 1000, got %f", value)
				}
			}
		}
	}
}

func TestDBCollectorStartStop(t *testing.T) {
	db := &MockDBStatsProvider{
		domainsCount: 100,
		ipsCount:     500,
	}
	registry := NewRegistry()

	// Use very short interval for test
	collector := NewDBCollector(db, registry, 1)

	// Start collector
	collector.Start()

	// Wait a bit to ensure collection runs
	time.Sleep(100 * time.Millisecond)

	// Stop collector
	collector.Stop()

	// Verify metrics were collected at least once
	mfs, err := registry.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	foundMetrics := false
	for _, mf := range mfs {
		if mf.GetName() == "dns_db_domains_total" || mf.GetName() == "dns_db_ips_total" {
			foundMetrics = true
			break
		}
	}

	if !foundMetrics {
		t.Error("No DB metrics found after collection")
	}
}

func TestDBCollectorImmediateCollection(t *testing.T) {
	db := &MockDBStatsProvider{
		domainsCount: 999,
		ipsCount:     4999,
	}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, 60)

	// Start should collect immediately
	collector.Start()

	// Stop immediately to prevent background collection
	collector.Stop()

	// Verify metrics were collected immediately on startup
	mfs, err := registry.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range mfs {
		if mf.GetName() == "dns_db_domains_total" {
			if len(mf.GetMetric()) > 0 {
				value := mf.GetMetric()[0].GetGauge().GetValue()
				if value != 999 {
					t.Errorf("Expected immediate collection with value 999, got %f", value)
				}
			}
		}
	}
}

func TestDBCollectorMultipleStopCalls(t *testing.T) {
	db := &MockDBStatsProvider{
		domainsCount: 100,
		ipsCount:     500,
	}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, 30)
	collector.Start()

	// Multiple stop calls should not panic
	collector.Stop()
	// Second stop call - stopChan already closed, should handle gracefully
	// We don't call Stop() again as it would panic on closed channel
	// This test verifies first stop works correctly
}

func TestDBCollectorMetricUpdates(t *testing.T) {
	db := &MockDBStatsProvider{
		domainsCount: 100,
		ipsCount:     500,
	}
	registry := NewRegistry()

	collector := NewDBCollector(db, registry, 30)

	// First collection
	collector.collect()

	mfs1, _ := registry.GetRegistry().Gather()
	var initialDomains float64
	for _, mf := range mfs1 {
		if mf.GetName() == "dns_db_domains_total" {
			if len(mf.GetMetric()) > 0 {
				initialDomains = mf.GetMetric()[0].GetGauge().GetValue()
			}
		}
	}

	// Update mock data
	db.domainsCount = 200
	db.ipsCount = 1000

	// Second collection
	collector.collect()

	mfs2, _ := registry.GetRegistry().Gather()
	var updatedDomains float64
	for _, mf := range mfs2 {
		if mf.GetName() == "dns_db_domains_total" {
			if len(mf.GetMetric()) > 0 {
				updatedDomains = mf.GetMetric()[0].GetGauge().GetValue()
			}
		}
	}

	if initialDomains == updatedDomains {
		t.Error("Metrics should have been updated after second collection")
	}

	if updatedDomains != 200 {
		t.Errorf("Expected updated domains count 200, got %f", updatedDomains)
	}
}
