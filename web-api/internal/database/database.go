package database

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"dns-collector-webapi/internal/models"
)

type Database struct {
	DB     *sql.DB
	config *dbConfig
}

type dbConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

func New(host string, port int, user, password, dbname, sslmode string) (*Database, error) {
	config := &dbConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: dbname,
		SSLMode:  sslmode,
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &Database{
		DB:     db,
		config: config,
	}, nil
}

func (db *Database) Close() error {
	if db.DB != nil {
		return db.DB.Close()
	}
	return nil
}

// GetStats retrieves DNS query statistics with filtering and sorting
func (db *Database) GetStats(filter models.StatsFilter) ([]models.DomainStat, int64, error) {
	query := "SELECT id, domain, client_ip, rtype, timestamp FROM domain_stat WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM domain_stat WHERE 1=1"
	args := []interface{}{}
	argPos := 1

	// Apply client IP filters
	if len(filter.ClientIPs) > 0 || filter.Subnet != "" {
		var ipConditions []string

		// Handle specific IPs
		if len(filter.ClientIPs) > 0 {
			placeholders := make([]string, len(filter.ClientIPs))
			for i, ip := range filter.ClientIPs {
				placeholders[i] = fmt.Sprintf("$%d", argPos)
				argPos++
				args = append(args, ip)
			}
			ipConditions = append(ipConditions, fmt.Sprintf("client_ip IN (%s)", strings.Join(placeholders, ",")))
		}

		// Handle subnet using PostgreSQL inet operators
		if filter.Subnet != "" {
			// Validate CIDR format
			_, _, err := net.ParseCIDR(filter.Subnet)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid subnet format: %w", err)
			}
			ipConditions = append(ipConditions, fmt.Sprintf("client_ip::inet << $%d::inet", argPos))
			argPos++
			args = append(args, filter.Subnet)
		}

		if len(ipConditions) > 0 {
			query += " AND (" + strings.Join(ipConditions, " OR ") + ")"
			countQuery += " AND (" + strings.Join(ipConditions, " OR ") + ")"
		}
	}

	// Apply date filters
	if !filter.DateFrom.IsZero() {
		query += fmt.Sprintf(" AND timestamp >= $%d", argPos)
		countQuery += fmt.Sprintf(" AND timestamp >= $%d", argPos)
		argPos++
		args = append(args, filter.DateFrom)
	}
	if !filter.DateTo.IsZero() {
		query += fmt.Sprintf(" AND timestamp <= $%d", argPos)
		countQuery += fmt.Sprintf(" AND timestamp <= $%d", argPos)
		argPos++
		args = append(args, filter.DateTo)
	}

	// Get total count
	var total int64
	err := db.DB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count stats: %w", err)
	}

	// Apply sorting
	validSortFields := map[string]bool{
		"id": true, "domain": true, "client_ip": true, "rtype": true, "timestamp": true,
	}
	sortBy := "timestamp"
	if filter.SortBy != "" && validSortFields[filter.SortBy] {
		sortBy = filter.SortBy
	}

	sortOrder := "DESC"
	if filter.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Apply pagination
	limit := filter.Limit
	if limit <= 0 {
		limit = 100 // Default limit
	}
	query += fmt.Sprintf(" LIMIT $%d", argPos)
	args = append(args, limit)
	argPos++

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []models.DomainStat
	for rows.Next() {
		var s models.DomainStat
		if err := rows.Scan(&s.ID, &s.Domain, &s.ClientIP, &s.RType, &s.Timestamp); err != nil {
			return nil, 0, fmt.Errorf("failed to scan stat: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, total, rows.Err()
}

// GetDomains retrieves domains with filtering and sorting
func (db *Database) GetDomains(filter models.DomainsFilter) ([]models.Domain, int64, error) {
	query := "SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time FROM domain WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM domain WHERE 1=1"
	args := []interface{}{}
	argPos := 1

	// Apply domain regex filter in SQL
	if filter.DomainRegex != "" {
		// Validate regex pattern to prevent ReDoS attacks
		if len(filter.DomainRegex) > 200 {
			return nil, 0, fmt.Errorf("regex pattern too long (max 200 characters)")
		}

		// Check for potentially dangerous patterns
		dangerousPatterns := []string{
			"(.*)*",    // Catastrophic backtracking
			"(.+)+",    // Catastrophic backtracking
			"(.*)+",    // Catastrophic backtracking
			"(.+)*",    // Catastrophic backtracking
		}
		for _, dangerous := range dangerousPatterns {
			if strings.Contains(filter.DomainRegex, dangerous) {
				return nil, 0, fmt.Errorf("regex pattern contains potentially dangerous construct: %s", dangerous)
			}
		}

		query += fmt.Sprintf(" AND domain ~ $%d", argPos)
		countQuery += fmt.Sprintf(" AND domain ~ $%d", argPos)
		argPos++
		args = append(args, filter.DomainRegex)
	}

	// Apply date filters
	if !filter.DateFrom.IsZero() {
		query += fmt.Sprintf(" AND time_insert >= $%d", argPos)
		countQuery += fmt.Sprintf(" AND time_insert >= $%d", argPos)
		argPos++
		args = append(args, filter.DateFrom)
	}
	if !filter.DateTo.IsZero() {
		query += fmt.Sprintf(" AND time_insert <= $%d", argPos)
		countQuery += fmt.Sprintf(" AND time_insert <= $%d", argPos)
		argPos++
		args = append(args, filter.DateTo)
	}

	// Get total count
	var total int64
	err := db.DB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count domains: %w", err)
	}

	// Apply sorting
	validSortFields := map[string]bool{
		"id": true, "domain": true, "time_insert": true,
		"resolv_count": true, "max_resolv": true, "last_resolv_time": true,
	}
	sortBy := "time_insert"
	if filter.SortBy != "" && validSortFields[filter.SortBy] {
		sortBy = filter.SortBy
	}

	sortOrder := "DESC"
	if filter.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Apply pagination
	limit := filter.Limit
	if limit <= 0 {
		limit = 100 // Default limit
	}
	query += fmt.Sprintf(" LIMIT $%d", argPos)
	args = append(args, limit)
	argPos++

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query domains: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var domains []models.Domain
	for rows.Next() {
		var d models.Domain
		if err := rows.Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime); err != nil {
			return nil, 0, fmt.Errorf("failed to scan domain: %w", err)
		}
		domains = append(domains, d)
	}

	return domains, total, rows.Err()
}

// GetDomainIPs retrieves all IP addresses for a specific domain
func (db *Database) GetDomainIPs(domainID int64) ([]models.IP, error) {
	query := "SELECT id, domain_id, ip, type, time FROM ip WHERE domain_id = $1 ORDER BY type, ip"

	rows, err := db.DB.Query(query, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query IPs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ips []models.IP
	for rows.Next() {
		var ip models.IP
		if err := rows.Scan(&ip.ID, &ip.DomainID, &ip.IP, &ip.Type, &ip.Time); err != nil {
			return nil, fmt.Errorf("failed to scan IP: %w", err)
		}
		ips = append(ips, ip)
	}

	return ips, rows.Err()
}

// GetDomainWithIPs retrieves a domain with all its IPs
func (db *Database) GetDomainWithIPs(domainID int64) (*models.Domain, error) {
	query := "SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time FROM domain WHERE id = $1"

	var d models.Domain
	err := db.DB.QueryRow(query, domainID).Scan(
		&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("domain not found")
		}
		return nil, fmt.Errorf("failed to query domain: %w", err)
	}

	// Get IPs
	ips, err := db.GetDomainIPs(domainID)
	if err != nil {
		return nil, err
	}
	d.IPs = ips

	return &d, nil
}

// GetDomainsWithIPs retrieves domains with all their IPs using bulk fetch to avoid N+1 queries
func (db *Database) GetDomainsWithIPs(filter models.DomainsFilter) ([]models.Domain, int64, error) {
	// First, get filtered domains
	domains, total, err := db.GetDomains(filter)
	if err != nil {
		return nil, 0, err
	}

	if len(domains) == 0 {
		return domains, total, nil
	}

	// Collect domain IDs and create map for quick lookup
	domainIDs := make([]interface{}, len(domains))
	domainMap := make(map[int64]*models.Domain)
	for i := range domains {
		domainIDs[i] = domains[i].ID
		domainMap[domains[i].ID] = &domains[i]
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(domainIDs))
	for i := range domainIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	// Bulk fetch all IPs in ONE query
	query := fmt.Sprintf(`
		SELECT id, domain_id, ip, type, time
		FROM ip
		WHERE domain_id IN (%s)
		ORDER BY domain_id, type, ip
	`, strings.Join(placeholders, ","))

	rows, err := db.DB.Query(query, domainIDs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch IPs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Map IPs to domains
	for rows.Next() {
		var ip models.IP
		if err := rows.Scan(&ip.ID, &ip.DomainID, &ip.IP, &ip.Type, &ip.Time); err != nil {
			return nil, 0, fmt.Errorf("failed to scan IP: %w", err)
		}
		if domain, ok := domainMap[ip.DomainID]; ok {
			domain.IPs = append(domain.IPs, ip)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return domains, total, nil
}

// GetExportList retrieves domains and their IPs filtered by domain regex
func (db *Database) GetExportList(domainRegex string, includeIPv4, includeIPv6, excludeSharedIPs bool) (*models.ExportList, error) {
	// Validate regex pattern
	if domainRegex == "" {
		return nil, fmt.Errorf("domain regex is required")
	}
	if len(domainRegex) > 200 {
		return nil, fmt.Errorf("regex pattern too long (max 200 characters)")
	}

	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"(.*)*",
		"(.+)+",
		"(.*)+",
		"(.+)*",
	}
	for _, dangerous := range dangerousPatterns {
		if strings.Contains(domainRegex, dangerous) {
			return nil, fmt.Errorf("regex pattern contains potentially dangerous construct: %s", dangerous)
		}
	}

	// Query to get unique domains matching the regex
	domainsQuery := `
		SELECT DISTINCT domain
		FROM domain
		WHERE domain ~ $1
		ORDER BY domain
	`

	rows, err := db.DB.Query(domainsQuery, domainRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, fmt.Errorf("failed to scan domain: %w", err)
		}
		domains = append(domains, domain)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// If neither IPv4 nor IPv6 is enabled, return early with just domains
	if !includeIPv4 && !includeIPv6 {
		return &models.ExportList{
			Domains: domains,
			IPv4:    []string{},
			IPv6:    []string{},
		}, nil
	}

	// Build IP query with type filtering and optional shared IP exclusion
	var ipsQuery string
	if excludeSharedIPs {
		// Complex query with CTE to exclude IPs shared between matched and non-matched domains
		ipsQuery = `
			WITH matched_ips AS (
				SELECT DISTINCT ip.ip, ip.type
				FROM ip
				INNER JOIN domain ON ip.domain_id = domain.id
				WHERE domain.domain ~ $1
			),
			non_matched_ips AS (
				SELECT DISTINCT ip.ip
				FROM ip
				INNER JOIN domain ON ip.domain_id = domain.id
				WHERE NOT (domain.domain ~ $1)
			)
			SELECT ip, type
			FROM matched_ips
			WHERE ip NOT IN (SELECT ip FROM non_matched_ips)
		`
	} else {
		// Simple query without shared IP exclusion
		ipsQuery = `
			SELECT DISTINCT ip.ip, ip.type
			FROM ip
			INNER JOIN domain ON ip.domain_id = domain.id
			WHERE domain.domain ~ $1
		`
	}

	// Add type filtering
	if includeIPv4 && !includeIPv6 {
		ipsQuery += " AND ip.type = 'ipv4'"
	} else if !includeIPv4 && includeIPv6 {
		ipsQuery += " AND ip.type = 'ipv6'"
	}
	// If both are true, no type filter needed

	ipsQuery += " ORDER BY type, ip"

	rows, err = db.DB.Query(ipsQuery, domainRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to query IPs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ipv4List []string
	var ipv6List []string

	for rows.Next() {
		var ip, ipType string
		if err := rows.Scan(&ip, &ipType); err != nil {
			return nil, fmt.Errorf("failed to scan IP: %w", err)
		}

		if ipType == "ipv4" {
			ipv4List = append(ipv4List, ip)
		} else if ipType == "ipv6" {
			ipv6List = append(ipv6List, ip)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &models.ExportList{
		Domains: domains,
		IPv4:    ipv4List,
		IPv6:    ipv6List,
	}, nil
}

// GetExcludedIPs retrieves IPs that are excluded from export due to being shared
// between matched and non-matched domains
func (db *Database) GetExcludedIPs(domainRegex string, includeIPv4, includeIPv6 bool) ([]models.ExcludedIPInfo, error) {
	// Validate regex pattern
	if domainRegex == "" {
		return nil, fmt.Errorf("domain regex is required")
	}
	if len(domainRegex) > 200 {
		return nil, fmt.Errorf("regex pattern too long (max 200 characters)")
	}

	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"(.*)*",
		"(.+)+",
		"(.*)+",
		"(.+)*",
	}
	for _, dangerous := range dangerousPatterns {
		if strings.Contains(domainRegex, dangerous) {
			return nil, fmt.Errorf("regex pattern contains potentially dangerous construct: %s", dangerous)
		}
	}

	// Build type filter condition
	typeFilter := ""
	if includeIPv4 && !includeIPv6 {
		typeFilter = " AND ip.type = 'ipv4'"
	} else if !includeIPv4 && includeIPv6 {
		typeFilter = " AND ip.type = 'ipv6'"
	}
	// If both are true or both are false, no type filter

	// Query to find IPs that appear in both matched and non-matched domains
	query := fmt.Sprintf(`
		WITH matched_domain_ips AS (
			SELECT DISTINCT ip.ip, domain.domain
			FROM ip
			INNER JOIN domain ON ip.domain_id = domain.id
			WHERE domain.domain ~ $1%s
		),
		non_matched_domain_ips AS (
			SELECT DISTINCT ip.ip, domain.domain
			FROM ip
			INNER JOIN domain ON ip.domain_id = domain.id
			WHERE NOT (domain.domain ~ $1)%s
		),
		shared_ips AS (
			SELECT DISTINCT m.ip
			FROM matched_domain_ips m
			INNER JOIN non_matched_domain_ips nm ON m.ip = nm.ip
		)
		SELECT
			s.ip,
			ARRAY_AGG(DISTINCT m.domain ORDER BY m.domain) AS matched_domains,
			ARRAY_AGG(DISTINCT nm.domain ORDER BY nm.domain) AS non_matched_domains
		FROM shared_ips s
		LEFT JOIN matched_domain_ips m ON s.ip = m.ip
		LEFT JOIN non_matched_domain_ips nm ON s.ip = nm.ip
		GROUP BY s.ip
		ORDER BY s.ip
	`, typeFilter, typeFilter)

	rows, err := db.DB.Query(query, domainRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to query excluded IPs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []models.ExcludedIPInfo
	for rows.Next() {
		var info models.ExcludedIPInfo
		var matchedDomains, nonMatchedDomains string

		// PostgreSQL array_agg returns comma-separated string in Go when using array_agg with text
		// We need to use pq.Array for proper array handling
		if err := rows.Scan(&info.IP, &matchedDomains, &nonMatchedDomains); err != nil {
			return nil, fmt.Errorf("failed to scan excluded IP info: %w", err)
		}

		// Parse the PostgreSQL array format {domain1,domain2,...}
		info.MatchedDomains = parsePostgreSQLArray(matchedDomains)
		info.NonMatchedDomains = parsePostgreSQLArray(nonMatchedDomains)

		result = append(result, info)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// parsePostgreSQLArray parses PostgreSQL array format {item1,item2,...} to Go slice
func parsePostgreSQLArray(s string) []string {
	// Remove leading { and trailing }
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return []string{}
	}
	if s[0] == '{' && s[len(s)-1] == '}' {
		s = s[1 : len(s)-1]
	}

	// Handle empty array
	if s == "" {
		return []string{}
	}

	// Split by comma
	items := strings.Split(s, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		// Remove quotes if present
		if len(item) > 1 && item[0] == '"' && item[len(item)-1] == '"' {
			item = item[1 : len(item)-1]
		}
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}
