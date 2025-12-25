# Changelog

All notable changes to DNS Collector will be documented in this file.

## [2.4.0] - 2025-12-24

### Added
- **IPv4/IPv6 Filtering for Export Lists**: Selective IP version export for pfSense
  - `include_ipv4` option: enable/disable IPv4 addresses in export (default: `true`)
  - `include_ipv6` option: enable/disable IPv6 addresses in export (default: `true`)
  - Useful for firewalls without IPv6 support or separate IPv4/IPv6 rules

- **Shared IP Exclusion**: Prevent blocking of CDN and cloud services
  - `exclude_shared_ips` option: exclude IPs used by both matched and non-matched domains (default: `false`)
  - Protects legitimate services sharing IPs with unwanted domains
  - Example: Prevents blocking Cloudflare IPs when blocking ads.cloudflare.com

- **Excluded IPs Analysis Endpoint**: Audit and debug IP filtering
  - `excluded_ips_endpoint` configuration option for separate analysis endpoint
  - Format: `IP | Matched Domains | Non-Matched Domains`
  - Helps make informed decisions about blocking shared IPs

- **Additional Static IPs from File**: Extend blocklists with external threat intelligence
  - `additional_ips_file` option: path to file with static IP addresses
  - Supports comments (#) and empty lines
  - Automatic deduplication with database IPs
  - Applies IPv4/IPv6 filtering
  - Maximum 100,000 lines per file

### Security
- Enhanced configuration validation for new optional parameters
- File path validation (must be absolute and in `/app/config/`)
- Limit on additional IPs file size (100,000 lines)

### Documentation
- Updated [`web-api/EXPORT_LISTS.md`](web-api/EXPORT_LISTS.md) with v2.4.0 features
- Added 5 practical examples covering all new features
- Added troubleshooting guide
- Created production config examples in `deploy/production/config/`
- Added `threat-intel-ips.txt.example` and `corporate-manual-blocks.txt.example`
- Added `README-additional-ips.md` with usage instructions

### Technical Details
- New database method: `GetExcludedIPs(domainRegex, includeIPv4, includeIPv6)` with PostgreSQL CTEs
- New utility package: `internal/utils/ip_parser.go` with file parsing and validation
- Updated `GetExportList()` with IPv4/IPv6 filtering and shared IP exclusion using CTEs
- New handler: `ExportExcludedIPs()` for excluded IPs analysis
- PostgreSQL array parsing helper: `parsePostgreSQLArray()` for ARRAY_AGG results
- Fixed closure capture bug in export list route registration
- Comprehensive tests for IP parser (4 test functions)
- All 88+ tests passing

### Configuration Examples

**IPv4-only export (firewall without IPv6 support):**
```yaml
export_lists:
  - name: "Ad Blocklist IPv4"
    endpoint: "/export/ads-ipv4"
    domain_regex: "^(ads|adservice|tracking)\\."
    include_domains: true
    include_ipv4: true
    include_ipv6: false
```

**Safe blocking with shared IP exclusion:**
```yaml
export_lists:
  - name: "Tracking Blocklist Safe"
    endpoint: "/export/tracking"
    excluded_ips_endpoint: "/export/tracking-excluded"
    domain_regex: "^(tracking|telemetry)\\."
    include_domains: true
    exclude_shared_ips: true
```

**Extended blocklist with threat intelligence:**
```yaml
export_lists:
  - name: "Malware Comprehensive"
    endpoint: "/export/malware"
    domain_regex: "\\.(malware|virus|trojan)\\."
    include_domains: false
    include_ipv4: true
    include_ipv6: true
    exclude_shared_ips: true
    additional_ips_file: "/app/config/threat-intel-ips.txt"
```

### Backward Compatibility
All new parameters are optional with sensible defaults. Existing configurations work without changes.


### Added
- **Excel Export**: New functionality to export DNS statistics and domains to Excel (XLSX) format
  - Export DNS statistics with full filtering support (client IPs, subnets, dates, sorting)
  - Export domains with IP addresses in dual-sheet Excel files
  - Full Excel formatting: bold headers, freeze panes, auto-filters, optimized column widths
  - Date formatting: `yyyy-mm-dd hh:mm:ss`
  - Export buttons integrated into filter blocks on Stats and Domains pages

- **API Endpoints**:
  - `GET /api/stats/export` - Export DNS query statistics to Excel
  - `GET /api/domains/export` - Export domains with IPs to Excel (2 sheets)

- **Performance Optimization**:
  - New `GetDomainsWithIPs()` database method using bulk IP fetching
  - Prevents N+1 query problem when exporting domains with IPs
  - Single SQL query with IN clause for fetching all IPs

- **Safety Features**:
  - 100,000 record limit for exports
  - HTTP 413 error response when dataset exceeds limit
  - Clear error messages for users when export fails or dataset is too large

### Frontend
- Export to Excel buttons on Stats and Domains pages
- Loading indicators during export operations
- Automatic file download with proper filename extraction
- Error handling with user-friendly messages

### Backend
- New `internal/excel` package with `Exporter` type
- `ExportStats()` method: generates single-sheet Excel for DNS statistics
- `ExportDomains()` method: generates two-sheet Excel (Domains + IP Addresses)
- Comprehensive unit tests for Excel generation (12 test cases)

### Technical Details
- Added dependency: `github.com/xuri/excelize/v2 v2.10.0`
- Excel Statistics sheet columns: ID, Domain, Client IP, Record Type, Timestamp
- Excel Domains sheet columns: ID, Domain, First Seen, Resolution Count, Max Resolutions, Last Resolved
- Excel IP Addresses sheet columns: Domain, IP Address, Type, Resolved At
- All 88 existing tests still passing plus new Excel export tests

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
