-- Initial database schema for DNS Collector
-- PostgreSQL migration script
-- Version: 1.0.0

-- Create domain table
CREATE TABLE IF NOT EXISTS domain (
    id SERIAL PRIMARY KEY,
    domain VARCHAR(255) UNIQUE NOT NULL,
    time_insert TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolv_count INTEGER DEFAULT 0,
    max_resolv INTEGER DEFAULT 10,
    last_resolv_time TIMESTAMP
);

-- Create ip table
CREATE TABLE IF NOT EXISTS ip (
    id SERIAL PRIMARY KEY,
    domain_id INTEGER NOT NULL REFERENCES domain(id) ON DELETE CASCADE,
    ip VARCHAR(45) NOT NULL,  -- IPv6 max length is 45 chars
    type VARCHAR(10) NOT NULL CHECK (type IN ('ipv4', 'ipv6')),
    time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create domain_stat table
CREATE TABLE IF NOT EXISTS domain_stat (
    id SERIAL PRIMARY KEY,
    domain VARCHAR(255) NOT NULL,
    client_ip VARCHAR(45) NOT NULL,
    rtype VARCHAR(20) NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add comments for documentation
COMMENT ON TABLE domain IS 'Stores unique domain names with resolution tracking';
COMMENT ON TABLE ip IS 'Stores resolved IP addresses for domains';
COMMENT ON TABLE domain_stat IS 'Stores statistics for DNS queries';

COMMENT ON COLUMN domain.resolv_count IS 'Number of times this domain has been resolved';
COMMENT ON COLUMN domain.max_resolv IS 'Maximum number of resolutions allowed for this domain';
COMMENT ON COLUMN ip.type IS 'Type of IP address: ipv4 or ipv6';
COMMENT ON COLUMN domain_stat.rtype IS 'Resolution type: cache or dns';
