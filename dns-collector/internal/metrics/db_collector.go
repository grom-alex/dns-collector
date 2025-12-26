package metrics

import (
	"log"
	"time"
)

// DBStatsProvider interface for getting database statistics.
type DBStatsProvider interface {
	GetDomainsCount() (int64, error)
	GetIPsCount() (int64, error)
}

// DBCollector periodically collects database statistics and updates metrics.
type DBCollector struct {
	db       DBStatsProvider
	registry *Registry
	interval time.Duration
	stopChan chan struct{}
	doneChan chan struct{}
}

// NewDBCollector creates a new database metrics collector.
func NewDBCollector(db DBStatsProvider, registry *Registry, intervalSeconds int) *DBCollector {
	if intervalSeconds <= 0 {
		intervalSeconds = 30 // default 30 seconds
	}
	return &DBCollector{
		db:       db,
		registry: registry,
		interval: time.Duration(intervalSeconds) * time.Second,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Start begins periodic collection of database metrics.
func (c *DBCollector) Start() {
	log.Printf("Starting DB metrics collector (interval: %v)", c.interval)

	// Collect immediately on startup
	c.collect()

	go c.run()
}

// Stop stops the database metrics collector.
func (c *DBCollector) Stop() {
	log.Println("Stopping DB metrics collector...")
	close(c.stopChan)
	<-c.doneChan
	log.Println("DB metrics collector stopped")
}

func (c *DBCollector) run() {
	defer close(c.doneChan)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.stopChan:
			return
		}
	}
}

func (c *DBCollector) collect() {
	// Collect domain count
	domainCount, err := c.db.GetDomainsCount()
	if err != nil {
		log.Printf("Error getting domains count: %v", err)
	} else {
		c.registry.DBDomainsTotal.Set(float64(domainCount))
	}

	// Collect IP count
	ipCount, err := c.db.GetIPsCount()
	if err != nil {
		log.Printf("Error getting IPs count: %v", err)
	} else {
		c.registry.DBIPsTotal.Set(float64(ipCount))
	}
}
