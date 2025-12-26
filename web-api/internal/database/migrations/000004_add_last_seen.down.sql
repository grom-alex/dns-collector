-- Rollback last_seen column addition
-- Version: 1.0.0

DROP INDEX IF EXISTS idx_ip_time_cleanup;
DROP INDEX IF EXISTS idx_domain_last_seen;
ALTER TABLE domain DROP COLUMN IF EXISTS last_seen;
