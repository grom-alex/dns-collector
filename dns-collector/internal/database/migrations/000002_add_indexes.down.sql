-- Rollback indexes
-- PostgreSQL migration script
-- Version: 1.0.0

-- Drop indexes for domain table
DROP INDEX IF EXISTS idx_domain_name;
DROP INDEX IF EXISTS idx_domain_resolv_count;
DROP INDEX IF EXISTS idx_domain_last_resolv_time;
DROP INDEX IF EXISTS idx_domain_time_insert;
DROP INDEX IF EXISTS idx_domain_resolv_lookup;

-- Drop indexes for ip table
DROP INDEX IF EXISTS idx_ip_domain_id;
DROP INDEX IF EXISTS idx_ip_type;
DROP INDEX IF EXISTS idx_ip_time;
DROP INDEX IF EXISTS idx_ip_address;
DROP INDEX IF EXISTS idx_ip_domain_type;

-- Drop indexes for domain_stat table
DROP INDEX IF EXISTS idx_domain_stat_timestamp;
DROP INDEX IF EXISTS idx_domain_stat_client_ip;
DROP INDEX IF EXISTS idx_domain_stat_domain;
DROP INDEX IF EXISTS idx_domain_stat_rtype;
DROP INDEX IF EXISTS idx_domain_stat_time_client;
DROP INDEX IF EXISTS idx_domain_stat_time_domain;
