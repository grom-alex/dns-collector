-- Partition domain_stat by month and replace the old indexes.
-- Retention drops whole old monthly partitions; the tail is deleted normally.
CREATE TABLE domain_stat_partitioned (
    id BIGSERIAL,
    domain TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    rtype TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL
) PARTITION BY RANGE (timestamp);

DO $$
DECLARE
    start_month date;
    end_month date;
    m date;
BEGIN
    SELECT date_trunc('month', COALESCE(min(timestamp), now()))::date,
           date_trunc('month', COALESCE(max(timestamp), now()))::date + interval '12 months'
    INTO start_month, end_month
    FROM domain_stat;

    FOR m IN SELECT generate_series(start_month, end_month, interval '1 month')::date LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS domain_stat_%s PARTITION OF domain_stat_partitioned FOR VALUES FROM (%L) TO (%L)',
            to_char(m, 'YYYY_MM'), m, (m + interval '1 month')::date
        );
    END LOOP;
END $$;

INSERT INTO domain_stat_partitioned (id, domain, client_ip, rtype, timestamp)
SELECT id, domain, client_ip, rtype, timestamp FROM domain_stat;

SELECT setval(pg_get_serial_sequence('domain_stat_partitioned', 'id'),
              COALESCE((SELECT max(id) FROM domain_stat_partitioned), 1), true);

CREATE INDEX idx_domain_stat_p_timestamp ON domain_stat_partitioned (timestamp);
CREATE INDEX idx_domain_stat_p_domain_ts ON domain_stat_partitioned (domain, timestamp);

ALTER TABLE domain_stat RENAME TO domain_stat_legacy;
ALTER TABLE domain_stat_partitioned RENAME TO domain_stat;
DROP TABLE domain_stat_legacy;

