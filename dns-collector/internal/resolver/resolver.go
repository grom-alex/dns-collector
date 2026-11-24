package resolver

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dns-collector/internal/config"
	"dns-collector/internal/database"
	"dns-collector/internal/metrics"
)

type Resolver struct {
	cfg           *config.Config
	db            *database.Database
	metrics       *metrics.Registry
	ticker        *time.Ticker
	stopCh        chan struct{}
	wg            sync.WaitGroup
	dnsConf       *net.Resolver
	activeWorkers int32
}

func NewResolver(cfg *config.Config, db *database.Database, m *metrics.Registry) *Resolver {
	return &Resolver{
		cfg:     cfg,
		db:      db,
		metrics: m,
		stopCh:  make(chan struct{}),
		dnsConf: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Duration(cfg.Resolver.TimeoutSeconds) * time.Second,
				}
				return d.DialContext(ctx, network, address)
			},
		},
	}
}

func (r *Resolver) Start() {
	interval := time.Duration(r.cfg.Resolver.IntervalSeconds) * time.Second
	r.ticker = time.NewTicker(interval)

	log.Printf("DNS resolver started with interval: %v", interval)

	// Run first resolution immediately
	go r.runResolution()

	// Start periodic resolution
	go func() {
		for {
			select {
			case <-r.ticker.C:
				go r.runResolution()
			case <-r.stopCh:
				return
			}
		}
	}()
}

func (r *Resolver) runResolution() {
	r.wg.Add(1)
	defer r.wg.Done()

	log.Println("Starting DNS resolution task")

	// Get domains that need to be resolved
	// We'll process them in batches using worker pool
	batchSize := r.cfg.Resolver.Workers * 10
	cyclicMode := r.cfg.Resolver.CyclicResolv
	cooldownMins := r.cfg.Resolver.ResolvCooldownMins

	domains, err := r.db.GetDomainsToResolve(batchSize, cyclicMode, cooldownMins)
	if err != nil {
		log.Printf("Error getting domains to resolve: %v", err)
		return
	}

	if len(domains) == 0 {
		log.Println("No domains to resolve")
		return
	}

	log.Printf("Found %d domains to resolve", len(domains))

	// Record batch size metric
	r.recordMetric(func(m *metrics.Registry) {
		m.ResolverBatchSize.Set(float64(len(domains)))
	})

	// Create worker pool
	domainCh := make(chan database.Domain, len(domains))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < r.cfg.Resolver.Workers; i++ {
		wg.Add(1)
		go r.worker(i+1, domainCh, &wg)
	}

	// Send domains to workers
	for _, domain := range domains {
		domainCh <- domain
	}
	close(domainCh)

	// Wait for all workers to finish
	wg.Wait()

	// Reset batch size after completion
	r.recordMetric(func(m *metrics.Registry) {
		m.ResolverBatchSize.Set(0)
		m.ResolverActiveWorkers.Set(0)
	})

	log.Println("DNS resolution task completed")
}

func (r *Resolver) worker(id int, domainCh <-chan database.Domain, wg *sync.WaitGroup) {
	defer wg.Done()

	// Track active workers
	atomic.AddInt32(&r.activeWorkers, 1)
	r.recordMetric(func(m *metrics.Registry) {
		m.ResolverActiveWorkers.Set(float64(atomic.LoadInt32(&r.activeWorkers)))
	})
	defer func() {
		atomic.AddInt32(&r.activeWorkers, -1)
		r.recordMetric(func(m *metrics.Registry) {
			m.ResolverActiveWorkers.Set(float64(atomic.LoadInt32(&r.activeWorkers)))
		})
	}()

	for domain := range domainCh {
		log.Printf("Worker %d: Resolving %s", id, domain.Domain)
		r.resolveDomain(domain)
	}
}

func (r *Resolver) resolveDomain(domain database.Domain) {
	timeout := time.Duration(r.cfg.Resolver.TimeoutSeconds) * time.Second

	ctx4, cancel4 := context.WithTimeout(context.Background(), timeout)
	ipv4Has, ipv4SyncErr := r.lookupAndSync(ctx4, "ip4", "ipv4", domain)
	cancel4()

	ctx6, cancel6 := context.WithTimeout(context.Background(), timeout)
	ipv6Has, ipv6SyncErr := r.lookupAndSync(ctx6, "ip6", "ipv6", domain)
	cancel6()

	// Update domain statistics even if resolution failed:
	// this prevents retrying non-existent domains indefinitely.
	cyclicMode := r.cfg.Resolver.CyclicResolv
	if err := r.db.UpdateDomainResolvStats(domain.ID, cyclicMode); err != nil {
		log.Printf("Error updating domain stats for %s: %v", domain.Domain, err)
	}

	status := "success"
	switch {
	case ipv4SyncErr != nil || ipv6SyncErr != nil:
		status = "db_error"
	case !ipv4Has && !ipv6Has:
		status = "no_results"
	}
	r.recordMetric(func(m *metrics.Registry) {
		m.ResolverDomainsProcessed.WithLabelValues(status).Inc()
	})
}

// lookupAndSync performs one DNS lookup and reconciles the stored IP set.
// It returns whether addresses were found and a database sync error, if any.
func (r *Resolver) lookupAndSync(ctx context.Context, network, ipType string, domain database.Domain) (bool, error) {
	start := time.Now()
	addrs, err := r.dnsConf.LookupIP(ctx, network, domain.Domain)
	duration := time.Since(start).Seconds()

	if err != nil {
		r.recordMetric(func(m *metrics.Registry) {
			m.ResolverLookups.WithLabelValues(ipType, "error").Inc()
			m.ResolverLookupDuration.WithLabelValues(ipType).Observe(duration)
		})
		// "No such host" is the normal case for missing A/AAAA records; don't spam logs.
		var dnsErr *net.DNSError
		if !errors.As(err, &dnsErr) || !dnsErr.IsNotFound {
			log.Printf("Error resolving %s for %s: %v", ipType, domain.Domain, err)
		}
		return false, nil
	}

	r.recordMetric(func(m *metrics.Registry) {
		m.ResolverLookups.WithLabelValues(ipType, "success").Inc()
		m.ResolverLookupDuration.WithLabelValues(ipType).Observe(duration)
	})

	ips := make([]string, 0, len(addrs))
	for _, ip := range addrs {
		ips = append(ips, ip.String())
	}

	refresh := time.Duration(r.cfg.Resolver.IPRefreshHours) * time.Hour
	if err := r.db.UpsertIPs(domain.ID, ipType, ips, refresh); err != nil {
		log.Printf("Error upserting %s for %s: %v", ipType, domain.Domain, err)
		return len(ips) > 0, err
	}
	return len(ips) > 0, nil
}

// resolveCNAME can be used if you want to follow CNAME records
func (r *Resolver) resolveCNAME(domain string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.cfg.Resolver.TimeoutSeconds)*time.Second)
	defer cancel()

	cname, err := r.dnsConf.LookupCNAME(ctx, domain)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(cname, "."), nil
}

func (r *Resolver) Stop() {
	log.Println("Stopping DNS resolver...")
	close(r.stopCh)
	if r.ticker != nil {
		r.ticker.Stop()
	}
	r.wg.Wait()
	log.Println("DNS resolver stopped")
}

// recordMetric safely records a metric if metrics are enabled.
func (r *Resolver) recordMetric(f func(m *metrics.Registry)) {
	if r.metrics != nil {
		f(r.metrics)
	}
}
