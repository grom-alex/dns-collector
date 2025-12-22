package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
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

type Domain struct {
	ID             int64
	Domain         string
	TimeInsert     time.Time
	ResolvCount    int
	MaxResolv      int
	LastResolvTime time.Time
}

type IPAddress struct {
	ID       int64
	DomainID int64
	IP       string
	Type     string
	Time     time.Time
}

type DomainStat struct {
	ID        int64
	Domain    string
	ClientIP  string
	RType     string
	Timestamp time.Time
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

	database := &Database{
		DB:     db,
		config: config,
	}

	if err := database.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return database, nil
}

func (db *Database) initSchema() error {
	// Create domain table
	domainSchema := `
	CREATE TABLE IF NOT EXISTS domain (
		id SERIAL PRIMARY KEY,
		domain TEXT NOT NULL UNIQUE,
		time_insert TIMESTAMP NOT NULL,
		resolv_count INTEGER NOT NULL DEFAULT 0,
		max_resolv INTEGER NOT NULL,
		last_resolv_time TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_domain_resolv ON domain(resolv_count, max_resolv);
	`

	if _, err := db.DB.Exec(domainSchema); err != nil {
		return fmt.Errorf("failed to create domain table: %w", err)
	}

	// Create ip table
	ipSchema := `
	CREATE TABLE IF NOT EXISTS ip (
		id SERIAL PRIMARY KEY,
		domain_id INTEGER NOT NULL,
		ip TEXT NOT NULL,
		type TEXT NOT NULL,
		time TIMESTAMP NOT NULL,
		UNIQUE(domain_id, ip),
		FOREIGN KEY(domain_id) REFERENCES domain(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_ip_domain ON ip(domain_id);
	`

	if _, err := db.DB.Exec(ipSchema); err != nil {
		return fmt.Errorf("failed to create ip table: %w", err)
	}

	// Create domain_stat table
	statSchema := `
	CREATE TABLE IF NOT EXISTS domain_stat (
		id SERIAL PRIMARY KEY,
		domain TEXT NOT NULL,
		client_ip TEXT NOT NULL,
		rtype TEXT NOT NULL,
		timestamp TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_domain_stat_timestamp ON domain_stat(timestamp);
	CREATE INDEX IF NOT EXISTS idx_domain_stat_domain ON domain_stat(domain);
	CREATE INDEX IF NOT EXISTS idx_domain_stat_client_ip ON domain_stat(client_ip);
	`

	if _, err := db.DB.Exec(statSchema); err != nil {
		return fmt.Errorf("failed to create domain_stat table: %w", err)
	}

	return nil
}

func (db *Database) Close() error {
	if db.DB != nil {
		return db.DB.Close()
	}
	return nil
}

// InsertOrGetDomain inserts a new domain or returns existing one
func (db *Database) InsertOrGetDomain(domain string, maxResolv int) (*Domain, error) {
	now := time.Now()

	// Use INSERT ... ON CONFLICT for upsert
	var d Domain
	err := db.DB.QueryRow(
		`INSERT INTO domain (domain, time_insert, resolv_count, max_resolv, last_resolv_time)
		VALUES ($1, $2, 0, $3, $4)
		ON CONFLICT (domain) DO NOTHING
		RETURNING id, domain, time_insert, resolv_count, max_resolv, last_resolv_time`,
		domain, now, maxResolv, now,
	).Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime)

	if err == sql.ErrNoRows {
		// Domain already exists, fetch it
		err = db.DB.QueryRow(
			`SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time
			FROM domain WHERE domain = $1`,
			domain,
		).Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing domain: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to insert domain: %w", err)
	}

	return &d, nil
}

// GetDomainsToResolve returns domains that need to be resolved
func (db *Database) GetDomainsToResolve(limit int) ([]Domain, error) {
	rows, err := db.DB.Query(
		`SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time
		FROM domain
		WHERE resolv_count < max_resolv
		ORDER BY last_resolv_time ASC
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime); err != nil {
			return nil, fmt.Errorf("failed to scan domain: %w", err)
		}
		domains = append(domains, d)
	}

	return domains, rows.Err()
}

// InsertOrUpdateIP inserts or updates an IP address
func (db *Database) InsertOrUpdateIP(domainID int64, ip, ipType string) error {
	now := time.Now()

	_, err := db.DB.Exec(
		`INSERT INTO ip (domain_id, ip, type, time)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT(domain_id, ip) DO UPDATE SET
			time = $5,
			type = $6`,
		domainID, ip, ipType, now, now, ipType,
	)
	if err != nil {
		return fmt.Errorf("failed to insert/update IP: %w", err)
	}

	return nil
}

// UpdateDomainResolvStats updates resolv_count and last_resolv_time
func (db *Database) UpdateDomainResolvStats(domainID int64) error {
	now := time.Now()

	_, err := db.DB.Exec(
		`UPDATE domain
		SET resolv_count = resolv_count + 1,
		    last_resolv_time = $1
		WHERE id = $2`,
		now, domainID,
	)
	if err != nil {
		return fmt.Errorf("failed to update domain stats: %w", err)
	}

	return nil
}

// InsertDomainStat inserts a new statistics record
func (db *Database) InsertDomainStat(domain, clientIP, rtype string) error {
	now := time.Now()

	_, err := db.DB.Exec(
		`INSERT INTO domain_stat (domain, client_ip, rtype, timestamp)
		VALUES ($1, $2, $3, $4)`,
		domain, clientIP, rtype, now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert domain stat: %w", err)
	}

	return nil
}

// DeleteOldStats deletes statistics records older than the specified number of days
func (db *Database) DeleteOldStats(retentionDays int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	result, err := db.DB.Exec(
		`DELETE FROM domain_stat WHERE timestamp < $1`,
		cutoffTime,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old stats: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return deleted, nil
}
