package cleanup

import (
	"log"
	"time"

	"dns-collector/internal/config"
	"dns-collector/internal/database"
)

type Service struct {
	db              *database.Database
	retentionDays   int
	cleanupInterval time.Duration
	stopChan        chan struct{}
	doneChan        chan struct{}
}

func NewService(cfg *config.Config, db *database.Database) *Service {
	return &Service{
		db:              db,
		retentionDays:   cfg.Retention.StatsDays,
		cleanupInterval: time.Duration(cfg.Retention.CleanupIntervalHours) * time.Hour,
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
	}
}

func (s *Service) Start() {
	log.Printf("Starting cleanup service (retention: %d days)", s.retentionDays)

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
	log.Printf("Running statistics cleanup (removing records older than %d days)...", s.retentionDays)

	deleted, err := s.db.DeleteOldStats(s.retentionDays)
	if err != nil {
		log.Printf("Error during cleanup: %v", err)
		return
	}

	if deleted > 0 {
		log.Printf("Cleanup completed: deleted %d old statistics records", deleted)
	} else {
		log.Println("Cleanup completed: no old records to delete")
	}
}
