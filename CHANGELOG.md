# Changelog

All notable changes to DNS Collector will be documented in this file.

## [1.0.1] - 2025-12-17

### Changed
- **Improved error logging**: Now logs the raw message content when JSON parsing fails
  - Before: `Error parsing JSON: unexpected end of JSON input`
  - After: `Error parsing JSON: unexpected end of JSON input, raw message: "{\"incomplete\":"`
  - This helps debug malformed UDP packets and identify the source of invalid messages

### Technical Details
- Modified `internal/server/udp.go:77` to include raw message in error output
- Uses `%q` format specifier to properly escape special characters in log output
- Helps identify issues with client implementations sending malformed JSON

## [1.0.0] - 2025-12-15

### Added
- Initial release of DNS Collector
- UDP server for receiving domain names in JSON format
- SQLite database storage with two separate databases:
  - `domains.db`: stores domains and resolved IP addresses
  - `stats.db`: logs all incoming UDP requests
- Periodic DNS resolution task with configurable interval
- Worker pool for parallel DNS resolution (IPv4 and IPv6)
- Docker support with multi-stage build
- Comprehensive documentation (README, INSTALL, ARCHITECTURE, etc.)
- Testing tools (test_client.py, monitor.sh)
- Database analysis scripts (check_db.go, check_stats.go)

### Features
- Configurable resolution interval and worker count
- Maximum resolution count per domain (max_resolv)
- Graceful shutdown with signal handling
- Health check support in Docker
- Resource limits in docker-compose
- Log rotation configuration
- IPv4 and IPv6 DNS resolution
- Unique constraint on (domain_id, ip) in ip table
- ON CONFLICT DO UPDATE pattern for IP insertion
