-- Performance tuning and security settings for DNS Collector
-- PostgreSQL migration script
-- Version: 1.0.0

-- Set statement timeout to prevent long-running queries (e.g., ReDoS)
-- Default: 30 seconds for regular queries
ALTER DATABASE dns_collector SET statement_timeout = '30s';

-- Set lock timeout to prevent deadlocks
ALTER DATABASE dns_collector SET lock_timeout = '10s';

-- Set idle in transaction timeout
ALTER DATABASE dns_collector SET idle_in_transaction_session_timeout = '60s';

-- Performance settings for better query execution
-- Increase work_mem for sorting and hash operations
ALTER DATABASE dns_collector SET work_mem = '16MB';

-- Increase maintenance_work_mem for VACUUM, CREATE INDEX
ALTER DATABASE dns_collector SET maintenance_work_mem = '128MB';

-- Enable JIT compilation for better performance on complex queries
ALTER DATABASE dns_collector SET jit = on;

-- Set effective_cache_size (should be ~75% of available RAM)
-- Adjust based on your server's memory
ALTER DATABASE dns_collector SET effective_cache_size = '512MB';

-- Random page cost (lower for SSD)
ALTER DATABASE dns_collector SET random_page_cost = 1.1;

-- Comments
COMMENT ON DATABASE dns_collector IS 'DNS Collector database with performance tuning and security settings';

-- Note: These settings apply to new connections only
-- To apply immediately, reconnect or run: SELECT pg_reload_conf();
