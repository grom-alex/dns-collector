package resolver

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"dns-collector/internal/config"
	"dns-collector/internal/database"
)

type Resolver struct {
	cfg     *config.Config
	db      *database.Database
	ticker  *time.Ticker
	stopCh  chan struct{}
	wg      sync.WaitGroup
	dnsConf *net.Resolver
}

func NewResolver(cfg *config.Config, db *database.Database) *Resolver {
	return &Resolver{
		cfg:    cfg,
		db:     db,
		stopCh: make(chan struct{}),
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

	log.Println("DNS resolution task completed")
}

func (r *Resolver) worker(id int, domainCh <-chan database.Domain, wg *sync.WaitGroup) {
	defer wg.Done()

	for domain := range domainCh {
		log.Printf("Worker %d: Resolving %s", id, domain.Domain)
		r.resolveDomain(domain)
	}
}

func (r *Resolver) resolveDomain(domain database.Domain) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.cfg.Resolver.TimeoutSeconds)*time.Second)
	defer cancel()

	hasResults := false

	// Resolve IPv4 addresses
	ipv4Addrs, err := r.dnsConf.LookupIP(ctx, "ip4", domain.Domain)
	if err != nil {
		log.Printf("Error resolving IPv4 for %s: %v", domain.Domain, err)
	} else {
		for _, ip := range ipv4Addrs {
			ipStr := ip.String()
			if err := r.db.InsertOrUpdateIP(domain.ID, ipStr, "ipv4"); err != nil {
				log.Printf("Error inserting IPv4 %s for domain %s: %v", ipStr, domain.Domain, err)
			} else {
				log.Printf("Resolved %s -> %s (IPv4)", domain.Domain, ipStr)
				hasResults = true
			}
		}
	}

	// Resolve IPv6 addresses
	ipv6Addrs, err := r.dnsConf.LookupIP(ctx, "ip6", domain.Domain)
	if err != nil {
		log.Printf("Error resolving IPv6 for %s: %v", domain.Domain, err)
	} else {
		for _, ip := range ipv6Addrs {
			ipStr := ip.String()
			if err := r.db.InsertOrUpdateIP(domain.ID, ipStr, "ipv6"); err != nil {
				log.Printf("Error inserting IPv6 %s for domain %s: %v", ipStr, domain.Domain, err)
			} else {
				log.Printf("Resolved %s -> %s (IPv6)", domain.Domain, ipStr)
				hasResults = true
			}
		}
	}

	// Update domain statistics even if resolution failed
	// This ensures we don't keep trying to resolve non-existent domains
	cyclicMode := r.cfg.Resolver.CyclicResolv
	if err := r.db.UpdateDomainResolvStats(domain.ID, cyclicMode); err != nil {
		log.Printf("Error updating domain stats for %s: %v", domain.Domain, err)
	}

	if !hasResults {
		log.Printf("No IP addresses resolved for %s", domain.Domain)
	}
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
