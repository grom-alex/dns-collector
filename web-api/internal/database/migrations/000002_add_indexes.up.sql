-- Performance indexes for DNS Collector
-- PostgreSQL migration script
-- Version: 1.0.0

-- Indexes for domain table
CREATE INDEX IF NOT EXISTS idx_domain_name ON domain(domain);
CREATE INDEX IF NOT EXISTS idx_domain_resolv_count ON domain(resolv_count) WHERE resolv_count < max_resolv;
CREATE INDEX IF NOT EXISTS idx_domain_last_resolv_time ON domain(last_resolv_time);
CREATE INDEX IF NOT EXISTS idx_domain_time_insert ON domain(time_insert);

-- Composite index for resolver worker queries
CREATE INDEX IF NOT EXISTS idx_domain_resolv_lookup ON domain(resolv_count, last_resolv_time)
    WHERE resolv_count < max_resolv;

-- Indexes for ip table
CREATE INDEX IF NOT EXISTS idx_ip_domain_id ON ip(domain_id);
CREATE INDEX IF NOT EXISTS idx_ip_type ON ip(type);
CREATE INDEX IF NOT EXISTS idx_ip_time ON ip(time);
CREATE INDEX IF NOT EXISTS idx_ip_address ON ip(ip);

-- Composite index for IP lookups by domain
CREATE INDEX IF NOT EXISTS idx_ip_domain_type ON ip(domain_id, type);

-- Indexes for domain_stat table (statistics queries)
CREATE INDEX IF NOT EXISTS idx_domain_stat_timestamp ON domain_stat(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_domain_stat_client_ip ON domain_stat(client_ip);
CREATE INDEX IF NOT EXISTS idx_domain_stat_domain ON domain_stat(domain);
CREATE INDEX IF NOT EXISTS idx_domain_stat_rtype ON domain_stat(rtype);

-- Composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_domain_stat_time_client ON domain_stat(timestamp DESC, client_ip);
CREATE INDEX IF NOT EXISTS idx_domain_stat_time_domain ON domain_stat(timestamp DESC, domain);

-- Partial index for recent stats (last 30 days)
-- Note: Cannot use NOW() in WHERE clause as it's not IMMUTABLE
-- Instead, use this index for all timestamp queries and let PostgreSQL optimize

-- Comments
COMMENT ON INDEX idx_domain_resolv_lookup IS 'Optimizes resolver worker queries for domains needing resolution';
