CREATE TABLE domain_stat_unpartitioned (
    id BIGSERIAL,
    domain TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    rtype TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL
);

INSERT INTO domain_stat_unpartitioned (id, domain, client_ip, rtype, timestamp)
SELECT id, domain, client_ip, rtype, timestamp FROM domain_stat;

SELECT setval(pg_get_serial_sequence('domain_stat_unpartitioned', 'id'),
              COALESCE((SELECT max(id) FROM domain_stat_unpartitioned), 1), true);

DROP TABLE domain_stat;
ALTER TABLE domain_stat_unpartitioned RENAME TO domain_stat;

ALTER TABLE domain_stat ADD PRIMARY KEY (id);
CREATE INDEX idx_domain_stat_timestamp ON domain_stat(timestamp DESC);
CREATE INDEX idx_domain_stat_time_domain ON domain_stat(timestamp DESC, domain);

