-- Add last_seen column to domain table
-- Tracks when domain was last requested by a client (via DNS query)
-- Used for TTL-based IP cleanup to protect inactive domains
-- Version: 1.0.0

-- Add last_seen column
ALTER TABLE domain ADD COLUMN IF NOT EXISTS last_seen TIMESTAMP;

-- Initialize last_seen for existing domains with current time
-- All existing domains are treated as "recently seen" at migration time
UPDATE domain SET last_seen = NOW()
WHERE last_seen IS NULL;

-- Add index for efficient cleanup queries
CREATE INDEX IF NOT EXISTS idx_domain_last_seen ON domain(last_seen);

-- Add index for IP time-based cleanup
CREATE INDEX IF NOT EXISTS idx_ip_time_cleanup ON ip(time);
