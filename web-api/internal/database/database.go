package database

import (
	"database/sql"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"dns-collector-webapi/internal/models"
)

type Database struct {
	DB *sql.DB
}

func New(host string, port int, user, password, dbname, sslmode string) (*Database, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &Database{DB: db}, nil
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

		// Handle subnet
		if filter.Subnet != "" {
			// We'll filter subnet in application layer
			ipConditions = append(ipConditions, "1=1")
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
	defer rows.Close()

	var stats []models.DomainStat
	var subnet *net.IPNet

	// Parse subnet if provided
	if filter.Subnet != "" {
		_, subnet, err = net.ParseCIDR(filter.Subnet)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid subnet format: %w", err)
		}
	}

	for rows.Next() {
		var s models.DomainStat
		if err := rows.Scan(&s.ID, &s.Domain, &s.ClientIP, &s.RType, &s.Timestamp); err != nil {
			return nil, 0, fmt.Errorf("failed to scan stat: %w", err)
		}

		// Filter by subnet if specified
		if subnet != nil {
			ip := net.ParseIP(s.ClientIP)
			if ip == nil || !subnet.Contains(ip) {
				continue
			}
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
	defer rows.Close()

	var domains []models.Domain
	var domainRegex *regexp.Regexp

	// Compile regex if provided
	if filter.DomainRegex != "" {
		domainRegex, err = regexp.Compile(filter.DomainRegex)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	for rows.Next() {
		var d models.Domain
		if err := rows.Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime); err != nil {
			return nil, 0, fmt.Errorf("failed to scan domain: %w", err)
		}

		// Filter by regex if specified
		if domainRegex != nil && !domainRegex.MatchString(d.Domain) {
			continue
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
	defer rows.Close()

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
