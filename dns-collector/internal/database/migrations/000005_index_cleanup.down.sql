-- Rollback for 000005: recreate the removed indexes and reset fillfactor.

CREATE INDEX IF NOT EXISTS idx_domain_name ON domain(domain);
CREATE INDEX IF NOT EXISTS idx_domain_resolv_count ON domain(resolv_count) WHERE resolv_count < max_resolv;
CREATE INDEX IF NOT EXISTS idx_domain_time_insert ON domain(time_insert);
CREATE INDEX IF NOT EXISTS idx_domain_resolv_lookup ON domain(resolv_count, last_resolv_time)
    WHERE resolv_count < max_resolv;
CREATE INDEX IF NOT EXISTS idx_domain_resolv ON domain(resolv_count, max_resolv);
CREATE INDEX IF NOT EXISTS idx_domain_last_seen ON domain(last_seen);

CREATE INDEX IF NOT EXISTS idx_ip_time ON ip(time);
CREATE INDEX IF NOT EXISTS idx_ip_time_cleanup ON ip(time);
CREATE INDEX IF NOT EXISTS idx_ip_domain ON ip(domain_id);

CREATE INDEX IF NOT EXISTS idx_domain_stat_time_client ON domain_stat(timestamp DESC, client_ip);
CREATE INDEX IF NOT EXISTS idx_domain_stat_client_ip ON domain_stat(client_ip);
CREATE INDEX IF NOT EXISTS idx_domain_stat_domain ON domain_stat(domain);
CREATE INDEX IF NOT EXISTS idx_domain_stat_rtype ON domain_stat(rtype);

ALTER TABLE domain RESET (fillfactor);
ALTER TABLE ip RESET (fillfactor);
