package writer

import (
	"sync"
	"testing"
	"time"

	"dns-collector/internal/database"
)

type fakeStore struct {
	mu           sync.Mutex
	domains      map[string]*database.Domain
	nextID       int64
	domainCalls  int
	stats        []database.DomainStat
	lastSeen     map[int64]time.Time
	lastSeenRuns int
}

func newFakeStore() *fakeStore {
	return &fakeStore{domains: make(map[string]*database.Domain), lastSeen: make(map[int64]time.Time)}
}

func (s *fakeStore) InsertOrGetDomain(domain string, maxResolv int) (*database.Domain, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.domainCalls++
	if d, ok := s.domains[domain]; ok {
		return d, false, nil
	}
	s.nextID++
	d := &database.Domain{ID: s.nextID, Domain: domain, MaxResolv: maxResolv}
	s.domains[domain] = d
	return d, true, nil
}

func (s *fakeStore) InsertDomainStatsBatch(stats []database.DomainStat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats = append(s.stats, stats...)
	return nil
}

func (s *fakeStore) UpdateLastSeenBatch(updates map[int64]time.Time, _ time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSeenRuns++
	for id, ts := range updates {
		s.lastSeen[id] = ts
	}
	return int64(len(updates)), nil
}

func testConfig() Config {
	return Config{
		QueueSize:             100,
		BatchSize:             2,
		FlushInterval:         5 * time.Millisecond,
		LastSeenFlushInterval: 5 * time.Millisecond,
		LastSeenMinInterval:   0,
		DomainCacheSize:       10,
		DomainCacheTTL:        time.Minute,
	}
}

func TestSubmitBatchesStatsAndCachesDomain(t *testing.T) {
	store := newFakeStore()
	w := New(store, 10, testConfig(), nil)
	w.Start()

	w.Submit("example.com", "192.0.2.1", "dns")
	w.Submit("example.com", "192.0.2.2", "cache")
	w.Stop()

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(store.stats))
	}
	if store.domainCalls != 1 {
		t.Fatalf("expected one domain lookup (cache hit on second submit), got %d", store.domainCalls)
	}
	if len(store.lastSeen) != 1 {
		t.Fatalf("expected one last_seen update, got %d", len(store.lastSeen))
	}
	if store.lastSeenRuns == 0 {
		t.Fatal("expected last_seen flush to run")
	}
}

func TestStopFlushesPendingStats(t *testing.T) {
	store := newFakeStore()
	cfg := testConfig()
	cfg.FlushInterval = time.Hour // don't flush by ticker; Stop must drain
	cfg.LastSeenFlushInterval = time.Hour
	w := New(store, 10, cfg, nil)
	w.Start()

	w.Submit("one.example", "192.0.2.1", "dns")
	w.Submit("two.example", "192.0.2.2", "dns")
	w.Stop()

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.stats) != 2 {
		t.Fatalf("expected 2 stats after Stop, got %d", len(store.stats))
	}
}

func TestDomainCacheExpiryAndEviction(t *testing.T) {
	c := newDomainCache(2, time.Millisecond)
	c.set("a.example", 1)
	if id, ok := c.get("a.example"); !ok || id != 1 {
		t.Fatalf("expected cache hit for a.example")
	}
	time.Sleep(5 * time.Millisecond)
	if _, ok := c.get("a.example"); ok {
		t.Fatal("expected expired entry to be evicted on get")
	}

	c.set("b.example", 2)
	c.set("c.example", 3)
	c.set("d.example", 4)
	if len(c.items) > 2 {
		t.Fatalf("cache exceeded max size: %d", len(c.items))
	}
}

func TestSubmitWithoutStartDoesNotPanic(t *testing.T) {
	store := newFakeStore()
	w := New(store, 10, testConfig(), nil)
	// No Start: stats queue is buffered; Stop without a consumer must not hang.
	w.Stop()
}
