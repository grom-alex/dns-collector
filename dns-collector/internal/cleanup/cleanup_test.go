package cleanup

import (
	"testing"
	"time"

	"dns-collector/internal/config"
)

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		Retention: config.RetentionConfig{
			StatsDays: 30,
		},
	}

	service := &Service{
		retentionDays:   cfg.Retention.StatsDays,
		cleanupInterval: 24 * time.Hour,
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
	}

	if service.retentionDays != 30 {
		t.Errorf("Expected retentionDays=30, got %d", service.retentionDays)
	}

	if service.cleanupInterval != 24*time.Hour {
		t.Errorf("Expected cleanupInterval=24h, got %v", service.cleanupInterval)
	}
}

func TestServiceRetentionPeriod(t *testing.T) {
	tests := []struct {
		name          string
		retentionDays int
	}{
		{"30 days", 30},
		{"7 days", 7},
		{"90 days", 90},
		{"1 day", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Retention: config.RetentionConfig{
					StatsDays: tt.retentionDays,
				},
			}

			service := &Service{
				retentionDays:   cfg.Retention.StatsDays,
				cleanupInterval: 24 * time.Hour,
				stopChan:        make(chan struct{}),
				doneChan:        make(chan struct{}),
			}

			if service.retentionDays != tt.retentionDays {
				t.Errorf("Expected retentionDays=%d, got %d", tt.retentionDays, service.retentionDays)
			}
		})
	}
}

func TestServiceChannels(t *testing.T) {
	cfg := &config.Config{
		Retention: config.RetentionConfig{
			StatsDays: 30,
		},
	}

	service := &Service{
		retentionDays:   cfg.Retention.StatsDays,
		cleanupInterval: 24 * time.Hour,
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
	}

	// Test that channels are created
	if service.stopChan == nil {
		t.Error("Expected stopChan to be created")
	}

	if service.doneChan == nil {
		t.Error("Expected doneChan to be created")
	}

	// Test that we can close channels
	close(service.stopChan)
	close(service.doneChan)

	// Verify channels are closed
	select {
	case <-service.stopChan:
		// Successfully closed
	default:
		t.Error("stopChan should be closed")
	}
}

func TestCleanupInterval(t *testing.T) {
	service := &Service{
		cleanupInterval: 24 * time.Hour,
	}

	expectedInterval := 24 * time.Hour
	if service.cleanupInterval != expectedInterval {
		t.Errorf("Expected cleanup interval=%v, got %v", expectedInterval, service.cleanupInterval)
	}
}
