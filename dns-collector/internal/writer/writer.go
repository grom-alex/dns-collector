package writer

import (
	"log"
	"sync"
	"time"

	"dns-collector/internal/database"
	"dns-collector/internal/metrics"
)

// Store is the subset of database operations used by the writer.
type Store interface {
	InsertOrGetDomain(domain string, maxResolv int) (*database.Domain, bool, error)
	InsertDomainStatsBatch(stats []database.DomainStat) error
	UpdateLastSeenBatch(updates map[int64]time.Time, minInterval time.Duration) (int64, error)
}

// Config controls batching, cache and throttling.
type Config struct {
	QueueSize             int
	BatchSize             int
	FlushInterval         time.Duration
	LastSeenFlushInterval time.Duration
	LastSeenMinInterval   time.Duration
	DomainCacheSize       int
	DomainCacheTTL        time.Duration
}

func DefaultConfig() Config {
	return Config{
		QueueSize:             10000,
		BatchSize:             500,
		FlushInterval:         time.Second,
		LastSeenFlushInterval: 60 * time.Second,
		LastSeenMinInterval:   5 * time.Minute,
		DomainCacheSize:       100000,
		DomainCacheTTL:        24 * time.Hour,
	}
}

func (c Config) withDefaults() Config {
	d := DefaultConfig()
	if c.QueueSize <= 0 {
		c.QueueSize = d.QueueSize
	}
	if c.BatchSize <= 0 {
		c.BatchSize = d.BatchSize
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = d.FlushInterval
	}
	if c.LastSeenFlushInterval <= 0 {
		c.LastSeenFlushInterval = d.LastSeenFlushInterval
	}
	if c.LastSeenMinInterval < 0 {
		c.LastSeenMinInterval = d.LastSeenMinInterval
	}
	if c.DomainCacheSize <= 0 {
		c.DomainCacheSize = d.DomainCacheSize
	}
	if c.DomainCacheTTL <= 0 {
		c.DomainCacheTTL = d.DomainCacheTTL
	}
	return c
}

// Writer buffers domain_stat inserts, throttles last_seen and caches domain IDs.
type Writer struct {
	store      Store
	maxResolv  int
	cfg        Config
	metrics    *metrics.Registry
	statsCh    chan database.DomainStat
	cache      *domainCache
	lastSeenMu sync.Mutex
	lastSeen   map[int64]time.Time
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func New(store Store, maxResolv int, cfg Config, m *metrics.Registry) *Writer {
	cfg = cfg.withDefaults()
	return &Writer{
		store:     store,
		maxResolv: maxResolv,
		cfg:       cfg,
		metrics:   m,
		statsCh:   make(chan database.DomainStat, cfg.QueueSize),
		cache:     newDomainCache(cfg.DomainCacheSize, cfg.DomainCacheTTL),
		lastSeen:  make(map[int64]time.Time),
		stopCh:    make(chan struct{}),
	}
}

func (w *Writer) Start() {
	w.wg.Add(2)
	go w.statsLoop()
	go w.lastSeenLoop()
}

// Submit accepts one parsed DNS event. It never blocks on a full queue: the event
// is dropped and counted instead, so a slow database cannot grow memory without bound.
func (w *Writer) Submit(domain, clientIP, rtype string) {
	now := time.Now()

	id, ok := w.cache.get(domain)
	if !ok {
		d, isNew, err := w.store.InsertOrGetDomain(domain, w.maxResolv)
		if err != nil {
			log.Printf("Error inserting domain %s: %v", domain, err)
		} else {
			id = d.ID
			w.cache.set(domain, id)
			if isNew {
				w.recordMetric(func(m *metrics.Registry) { m.ServerNewDomains.Inc() })
			}
		}
	}

	stat := database.DomainStat{
		Domain:    domain,
		ClientIP:  clientIP,
		RType:     rtype,
		Timestamp: now,
	}
	select {
	case w.statsCh <- stat:
	default:
		w.recordMetric(func(m *metrics.Registry) {
			m.ServerMessagesDropped.WithLabelValues("stats_queue_full").Inc()
		})
		return
	}

	if id != 0 {
		w.lastSeenMu.Lock()
		w.lastSeen[id] = now
		w.lastSeenMu.Unlock()
	}
}

// Stop flushes pending stats/last_seen and stops background loops.
func (w *Writer) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *Writer) statsLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.flushStats(false)
		case <-w.stopCh:
			w.flushStats(true)
			return
		}
	}
}

func (w *Writer) flushStats(final bool) {
	for {
		batch := make([]database.DomainStat, 0, w.cfg.BatchSize)
		for len(batch) < w.cfg.BatchSize {
			select {
			case stat := <-w.statsCh:
				batch = append(batch, stat)
			default:
				// Drain what is available now.
				if len(batch) > 0 {
					w.writeStats(batch)
				}
				if !final {
					return
				}
				// On shutdown, keep draining until the channel is empty.
				if len(w.statsCh) == 0 {
					return
				}
				continue
			}
		}
		w.writeStats(batch)
		if !final {
			return
		}
	}
}

func (w *Writer) writeStats(batch []database.DomainStat) {
	if len(batch) == 0 {
		return
	}
	if err := w.store.InsertDomainStatsBatch(batch); err != nil {
		log.Printf("Error inserting stats batch (%d rows): %v", len(batch), err)
	}
}

func (w *Writer) lastSeenLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.cfg.LastSeenFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.flushLastSeen()
		case <-w.stopCh:
			w.flushLastSeen()
			return
		}
	}
}

func (w *Writer) flushLastSeen() {
	w.lastSeenMu.Lock()
	if len(w.lastSeen) == 0 {
		w.lastSeenMu.Unlock()
		return
	}
	updates := w.lastSeen
	w.lastSeen = make(map[int64]time.Time)
	w.lastSeenMu.Unlock()

	if _, err := w.store.UpdateLastSeenBatch(updates, w.cfg.LastSeenMinInterval); err != nil {
		log.Printf("Error updating last_seen (%d rows): %v", len(updates), err)
		// Preserve the newest timestamp for a retry on the next flush.
		w.lastSeenMu.Lock()
		for id, ts := range updates {
			if old, ok := w.lastSeen[id]; !ok || ts.After(old) {
				w.lastSeen[id] = ts
			}
		}
		w.lastSeenMu.Unlock()
	}
}

func (w *Writer) recordMetric(f func(m *metrics.Registry)) {
	if w.metrics != nil {
		f(w.metrics)
	}
}

type cacheEntry struct {
	id      int64
	expires time.Time
}

type domainCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
	max   int
	ttl   time.Duration
}

func newDomainCache(max int, ttl time.Duration) *domainCache {
	return &domainCache{items: make(map[string]cacheEntry), max: max, ttl: ttl}
}

func (c *domainCache) get(domain string) (int64, bool) {
	now := time.Now()
	c.mu.RLock()
	entry, ok := c.items[domain]
	c.mu.RUnlock()
	if !ok {
		return 0, false
	}
	if !entry.expires.After(now) {
		c.mu.Lock()
		delete(c.items, domain)
		c.mu.Unlock()
		return 0, false
	}
	return entry.id, true
}

func (c *domainCache) set(domain string, id int64) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) >= c.max {
		for key, entry := range c.items {
			if !entry.expires.After(now) {
				delete(c.items, key)
			}
		}
		if len(c.items) >= c.max {
			// Evict arbitrary entries down to 90% capacity; domains are cheap to reload.
			target := c.max * 9 / 10
			for key := range c.items {
				delete(c.items, key)
				if len(c.items) <= target {
					break
				}
			}
		}
	}
	c.items[domain] = cacheEntry{id: id, expires: now.Add(c.ttl)}
}
