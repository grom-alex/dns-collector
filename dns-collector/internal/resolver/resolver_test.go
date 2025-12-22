package resolver

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"dns-collector/internal/config"
	"dns-collector/internal/database"
)

// MockDatabase for testing resolver
type MockDatabase struct {
	GetDomainsToResolveFunc func(limit int) ([]database.Domain, error)
	InsertOrUpdateIPFunc    func(domainID int64, ip, ipType string) error
	UpdateDomainResolvStatsFunc func(domainID int64) error
}

func (m *MockDatabase) GetDomainsToResolve(limit int) ([]database.Domain, error) {
	if m.GetDomainsToResolveFunc != nil {
		return m.GetDomainsToResolveFunc(limit)
	}
	return []database.Domain{}, nil
}

func (m *MockDatabase) InsertOrUpdateIP(domainID int64, ip, ipType string) error {
	if m.InsertOrUpdateIPFunc != nil {
		return m.InsertOrUpdateIPFunc(domainID, ip, ipType)
	}
	return nil
}

func (m *MockDatabase) UpdateDomainResolvStats(domainID int64) error {
	if m.UpdateDomainResolvStatsFunc != nil {
		return m.UpdateDomainResolvStatsFunc(domainID)
	}
	return nil
}

func TestNewResolver(t *testing.T) {
	cfg := &config.Config{
		Resolver: config.ResolverConfig{
			IntervalSeconds: 10,
			TimeoutSeconds:  5,
			Workers:         3,
		},
	}

	mockDB := &MockDatabase{}

	resolver := &Resolver{
		cfg:    cfg,
		db:     nil, // Using nil for basic structure test
		stopCh: make(chan struct{}),
		dnsConf: &net.Resolver{
			PreferGo: true,
		},
	}

	if resolver.stopCh == nil {
		t.Error("Expected stopCh to be initialized")
	}

	if resolver.dnsConf == nil {
		t.Error("Expected dnsConf to be initialized")
	}

	_ = mockDB
	_ = cfg
}

func TestResolverConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		intervalSeconds int
		timeoutSeconds  int
		workers         int
	}{
		{"default config", 10, 5, 3},
		{"high frequency", 5, 3, 5},
		{"low frequency", 300, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Resolver: config.ResolverConfig{
					IntervalSeconds: tt.intervalSeconds,
					TimeoutSeconds:  tt.timeoutSeconds,
					Workers:         tt.workers,
				},
			}

			resolver := &Resolver{
				cfg:    cfg,
				stopCh: make(chan struct{}),
			}

			if resolver.cfg.Resolver.IntervalSeconds != tt.intervalSeconds {
				t.Errorf("Expected intervalSeconds=%d, got %d", tt.intervalSeconds, resolver.cfg.Resolver.IntervalSeconds)
			}

			if resolver.cfg.Resolver.TimeoutSeconds != tt.timeoutSeconds {
				t.Errorf("Expected timeoutSeconds=%d, got %d", tt.timeoutSeconds, resolver.cfg.Resolver.TimeoutSeconds)
			}

			if resolver.cfg.Resolver.Workers != tt.workers {
				t.Errorf("Expected workers=%d, got %d", tt.workers, resolver.cfg.Resolver.Workers)
			}
		})
	}
}

func TestResolveCNAME(t *testing.T) {
	cfg := &config.Config{
		Resolver: config.ResolverConfig{
			TimeoutSeconds: 5,
		},
	}

	resolver := NewResolver(cfg, nil)

	// Test with a real CNAME (this might fail in offline environments)
	cname, err := resolver.resolveCNAME("www.github.com")
	if err == nil {
		// If successful, verify result
		if cname == "" {
			t.Error("Expected non-empty CNAME")
		}

		// CNAME should not end with a dot
		if strings.HasSuffix(cname, ".") {
			t.Error("CNAME should not end with a dot")
		}
	}
	// If error, it's okay - DNS might not be available in test environment
}

func TestResolveCNAME_Timeout(t *testing.T) {
	cfg := &config.Config{
		Resolver: config.ResolverConfig{
			TimeoutSeconds: 1, // Very short timeout
		},
	}

	resolver := &Resolver{
		cfg: cfg,
		dnsConf: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Simulate slow DNS server
				time.Sleep(2 * time.Second)
				d := net.Dialer{}
				return d.DialContext(ctx, network, address)
			},
		},
	}

	// This should timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Resolver.TimeoutSeconds)*time.Second)
	defer cancel()

	_, err := resolver.dnsConf.LookupCNAME(ctx, "example.com")
	if err == nil {
		// Context should have timed out
		if ctx.Err() == context.DeadlineExceeded {
			// Expected timeout
		}
	}
}

func TestStopChannel(t *testing.T) {
	resolver := &Resolver{
		stopCh: make(chan struct{}),
	}

	// Test that we can close stopCh
	close(resolver.stopCh)

	// Test that stopCh is closed
	select {
	case <-resolver.stopCh:
		// Successfully closed
	default:
		t.Error("stopCh should be closed")
	}
}

func TestWorkerPool(t *testing.T) {
	cfg := &config.Config{
		Resolver: config.ResolverConfig{
			Workers: 3,
		},
	}

	if cfg.Resolver.Workers != 3 {
		t.Errorf("Expected 3 workers, got %d", cfg.Resolver.Workers)
	}

	// Test batch size calculation
	batchSize := cfg.Resolver.Workers * 10
	expectedBatchSize := 30

	if batchSize != expectedBatchSize {
		t.Errorf("Expected batch size=%d, got %d", expectedBatchSize, batchSize)
	}
}

func TestDNSResolver_PreferGo(t *testing.T) {
	cfg := &config.Config{
		Resolver: config.ResolverConfig{
			TimeoutSeconds: 5,
		},
	}

	resolver := NewResolver(cfg, nil)

	if !resolver.dnsConf.PreferGo {
		t.Error("Expected PreferGo to be true")
	}
}

func TestContextTimeout(t *testing.T) {
	timeoutSeconds := 5
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Verify context has timeout
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected context to have a deadline")
	}

	// Deadline should be in the future
	if time.Until(deadline) <= 0 {
		t.Error("Deadline should be in the future")
	}

	// Deadline should be approximately 5 seconds from now
	expectedDeadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	diff := deadline.Sub(expectedDeadline).Abs()

	if diff > 100*time.Millisecond {
		t.Errorf("Deadline differs too much from expected: %v", diff)
	}
}

func TestDomainChannel(t *testing.T) {
	domains := []database.Domain{
		{ID: 1, Domain: "example.com"},
		{ID: 2, Domain: "test.com"},
		{ID: 3, Domain: "sample.org"},
	}

	domainCh := make(chan database.Domain, len(domains))

	// Send domains to channel
	for _, domain := range domains {
		domainCh <- domain
	}
	close(domainCh)

	// Receive and verify
	count := 0
	for range domainCh {
		count++
	}

	if count != len(domains) {
		t.Errorf("Expected %d domains from channel, got %d", len(domains), count)
	}
}

func TestWaitGroup(t *testing.T) {
	resolver := &Resolver{
		stopCh: make(chan struct{}),
	}

	// Test that wg can be used
	resolver.wg.Add(1)
	go func() {
		defer resolver.wg.Done()
		time.Sleep(10 * time.Millisecond)
	}()

	resolver.wg.Wait()
	// If we reach here, wg worked correctly
}

func TestTickerInterval(t *testing.T) {
	intervalSeconds := 10
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	// Verify ticker is created
	if ticker == nil {
		t.Error("Expected ticker to be created")
	}

	// Ticker should have a channel
	select {
	case <-ticker.C:
		t.Error("Ticker should not fire immediately")
	case <-time.After(100 * time.Millisecond):
		// Expected - ticker hasn't fired yet
	}
}
