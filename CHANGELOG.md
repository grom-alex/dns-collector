# Changelog

All notable changes to DNS Collector will be documented in this file.

## [2.3.1] - 2025-12-23

### Fixed
- **Export Lists**: Fixed domain name formatting for pfSense compatibility
  - Removed trailing dots from FQDN format in export output
  - Example: `gecko16-normal-c-useast1a.tiktokv.com.` → `gecko16-normal-c-useast1a.tiktokv.com`
  - Ensures proper compatibility with pfSense firewall alias tables

### Added
- **Testing**: New test `TestExportList_RemoveTrailingDot` to verify dot removal
- **Testing**: Updated `TestExportList_Success` with FQDN format in mock data

### Technical Details
- Modified `web-api/internal/handlers/handlers.go:203` to strip trailing dots using `strings.TrimSuffix`
- All 88 tests passing with 85.4% handler coverage

## [2.3.0] - 2025-12-22

### Added
- **Export Lists for pfSense**: New functionality to export IP addresses and domains in plain text format
  - HTTP endpoints returning plain text (Content-Type: text/plain)
  - Filtering by regex for domain names (PostgreSQL regex syntax)
  - Configurable domain inclusion (domains + IPs or IPs only)
  - Support for multiple lists with different criteria
  - Automatic sorting: domains → IPv4 → IPv6
  - HTTP caching headers (Cache-Control: public, max-age=300)

- **Security Features**:
  - ReDoS protection with dangerous pattern detection: `(.*)*`, `(.+)+`, `(.*)+`, `(.+)*`
  - Regex length limit (200 characters)
  - Configuration validation at startup (duplicate detection, required fields)

- **Documentation**:
  - New `web-api/EXPORT_LISTS.md` with pfSense integration guide
  - New `.claude/DEVELOPMENT_WORKFLOW.md` for production deployment process
  - Configuration examples in dev and production configs

- **Testing**: 22 new tests added
  - `TestExportList_Success`, `TestExportList_IPsOnly`, `TestExportList_EmptyResults`
  - `TestExportList_DatabaseError`, `TestExportList_OnlyIPv4`
  - Validation tests: `TestValidateExportLists_*` (9 tests)
  - Config loading tests: `TestLoadConfig_*` (5 tests)
  - Database validation tests: `TestGetExportList_*` (8 tests)

### Technical Details
- New database method: `GetExportList(domainRegex string)`
- New HTTP handler: `ExportList(c *gin.Context, domainRegex string, includeDomains bool)`
- New configuration structure: `ExportListConfig` with validation
- Dynamic endpoint registration based on configuration
- Defensive programming: panic prevention, empty result logging

### Configuration
```yaml
export_lists:
  - name: "Example Domain List"
    endpoint: "/export/example"
    domain_regex: "^example\\.com$"
    include_domains: true
```

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
