package cleanup

import (
	"log"
	"time"

	"dns-collector/internal/config"
	"dns-collector/internal/database"
	"dns-collector/internal/metrics"
)

type Service struct {
	db              *database.Database
	metrics         *metrics.Registry
	retentionDays   int
	ipTTLDays       int
	cleanupInterval time.Duration
	stopChan        chan struct{}
	doneChan        chan struct{}
}

func NewService(cfg *config.Config, db *database.Database, m *metrics.Registry) *Service {
	return &Service{
		db:              db,
		metrics:         m,
		retentionDays:   cfg.Retention.StatsDays,
		ipTTLDays:       cfg.Retention.IPTTLDays,
		cleanupInterval: time.Duration(cfg.Retention.CleanupIntervalHours) * time.Hour,
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
	}
}

func (s *Service) Start() {
	log.Printf("Starting cleanup service (stats retention: %d days, IP TTL: %d days)", s.retentionDays, s.ipTTLDays)

	// Run cleanup immediately on startup
	s.cleanup()

	go s.run()
}

func (s *Service) Stop() {
	log.Println("Stopping cleanup service...")
	close(s.stopChan)
	<-s.doneChan
	log.Println("Cleanup service stopped")
}

func (s *Service) run() {
	defer close(s.doneChan)

	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopChan:
			return
		}
	}
}

func (s *Service) cleanup() {
	start := time.Now()
	log.Printf("Running cleanup (stats: %d days, IP TTL: %d days)...", s.retentionDays, s.ipTTLDays)

	// Record cleanup run
	s.recordMetric(func(m *metrics.Registry) {
		m.CleanupRuns.Inc()
	})

	// 1. Cleanup old statistics
	statsDeleted, err := s.db.DeleteOldStats(s.retentionDays)
	if err != nil {
		log.Printf("Error during stats cleanup: %v", err)
	} else if statsDeleted > 0 {
		log.Printf("Stats cleanup: deleted %d old records", statsDeleted)
	}

	// Record stats cleanup metrics
	s.recordMetric(func(m *metrics.Registry) {
		m.CleanupStatsDeleted.Add(float64(statsDeleted))
	})

	// 2. Cleanup expired IP addresses (only for active domains)
	if s.ipTTLDays > 0 {
		ipsDeleted, err := s.db.DeleteExpiredIPs(s.ipTTLDays)
		if err != nil {
			log.Printf("Error during IP cleanup: %v", err)
		} else if ipsDeleted > 0 {
			log.Printf("IP cleanup: deleted %d expired IP addresses", ipsDeleted)
		}

		// Record IP cleanup metrics
		s.recordMetric(func(m *metrics.Registry) {
			m.CleanupIPsDeleted.Add(float64(ipsDeleted))
		})
	}

	// Record cleanup duration
	s.recordMetric(func(m *metrics.Registry) {
		m.CleanupDuration.Observe(time.Since(start).Seconds())
	})

	log.Println("Cleanup completed")
}

// recordMetric safely records a metric if metrics are enabled.
func (s *Service) recordMetric(f func(m *metrics.Registry)) {
	if s.metrics != nil {
		f(s.metrics)
	}
}
