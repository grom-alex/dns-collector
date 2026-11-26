-- Drop the resolver-time index so domain counter updates can use HOT.
-- Resolver query on ~16k-domain table is a cheap seq scan + top-N sort.
DROP INDEX IF EXISTS idx_domain_last_resolv_time;
