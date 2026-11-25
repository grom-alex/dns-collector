-- Stage 2: drop unused/duplicate indexes and set fillfactor for HOT updates.
-- Production applies the DROP INDEX statements manually with CONCURRENTLY;
-- this migration is a no-op there because the indexes are already gone.

DROP INDEX IF EXISTS idx_domain_name;
DROP INDEX IF EXISTS idx_domain_time_insert;
DROP INDEX IF EXISTS idx_domain_resolv_lookup;
DROP INDEX IF EXISTS idx_domain_resolv_count;
DROP INDEX IF EXISTS idx_domain_resolv;
DROP INDEX IF EXISTS idx_domain_last_seen;

DROP INDEX IF EXISTS idx_ip_time;
DROP INDEX IF EXISTS idx_ip_time_cleanup;
DROP INDEX IF EXISTS idx_ip_domain;

DROP INDEX IF EXISTS idx_domain_stat_time_client;
DROP INDEX IF EXISTS idx_domain_stat_client_ip;
DROP INDEX IF EXISTS idx_domain_stat_domain;
DROP INDEX IF EXISTS idx_domain_stat_rtype;

ALTER TABLE domain SET (fillfactor = 70);
ALTER TABLE ip SET (fillfactor = 70);
