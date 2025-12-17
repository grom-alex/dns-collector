package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	DomainsDB *sql.DB
	StatsDB   *sql.DB
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

func New(domainsDBPath, statsDBPath string) (*Database, error) {
	domainsDB, err := sql.Open("sqlite3", domainsDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open domains database: %w", err)
	}

	statsDB, err := sql.Open("sqlite3", statsDBPath)
	if err != nil {
		domainsDB.Close()
		return nil, fmt.Errorf("failed to open stats database: %w", err)
	}

	db := &Database{
		DomainsDB: domainsDB,
		StatsDB:   statsDB,
	}

	if err := db.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

func (db *Database) initSchema() error {
	// Create domain table
	domainSchema := `
	CREATE TABLE IF NOT EXISTS domain (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain TEXT NOT NULL UNIQUE,
		time_insert DATETIME NOT NULL,
		resolv_count INTEGER NOT NULL DEFAULT 0,
		max_resolv INTEGER NOT NULL,
		last_resolv_time DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_domain_resolv ON domain(resolv_count, max_resolv);
	`

	if _, err := db.DomainsDB.Exec(domainSchema); err != nil {
		return fmt.Errorf("failed to create domain table: %w", err)
	}

	// Create ip table
	ipSchema := `
	CREATE TABLE IF NOT EXISTS ip (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_id INTEGER NOT NULL,
		ip TEXT NOT NULL,
		type TEXT NOT NULL,
		time DATETIME NOT NULL,
		UNIQUE(domain_id, ip),
		FOREIGN KEY(domain_id) REFERENCES domain(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_ip_domain ON ip(domain_id);
	`

	if _, err := db.DomainsDB.Exec(ipSchema); err != nil {
		return fmt.Errorf("failed to create ip table: %w", err)
	}

	// Create domain_stat table
	statSchema := `
	CREATE TABLE IF NOT EXISTS domain_stat (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain TEXT NOT NULL,
		client_ip TEXT NOT NULL,
		rtype TEXT NOT NULL,
		timestamp DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_domain_stat_timestamp ON domain_stat(timestamp);
	CREATE INDEX IF NOT EXISTS idx_domain_stat_domain ON domain_stat(domain);
	`

	if _, err := db.StatsDB.Exec(statSchema); err != nil {
		return fmt.Errorf("failed to create domain_stat table: %w", err)
	}

	return nil
}

func (db *Database) Close() error {
	var err1, err2 error
	if db.DomainsDB != nil {
		err1 = db.DomainsDB.Close()
	}
	if db.StatsDB != nil {
		err2 = db.StatsDB.Close()
	}
	if err1 != nil {
		return err1
	}
	return err2
}

// InsertOrGetDomain inserts a new domain or returns existing one
func (db *Database) InsertOrGetDomain(domain string, maxResolv int) (*Domain, error) {
	now := time.Now()

	// Try to insert
	result, err := db.DomainsDB.Exec(
		`INSERT OR IGNORE INTO domain (domain, time_insert, resolv_count, max_resolv, last_resolv_time)
		VALUES (?, ?, 0, ?, ?)`,
		domain, now, maxResolv, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert domain: %w", err)
	}

	// Get the domain record
	var d Domain
	err = db.DomainsDB.QueryRow(
		`SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time
		FROM domain WHERE domain = ?`,
		domain,
	).Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}

	// Check if it was a new insert
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		d.TimeInsert = now
		d.LastResolvTime = now
	}

	return &d, nil
}

// GetDomainsToResolve returns domains that need to be resolved
func (db *Database) GetDomainsToResolve(limit int) ([]Domain, error) {
	rows, err := db.DomainsDB.Query(
		`SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time
		FROM domain
		WHERE resolv_count < max_resolv
		ORDER BY last_resolv_time ASC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains: %w", err)
	}
	defer rows.Close()

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

	_, err := db.DomainsDB.Exec(
		`INSERT INTO ip (domain_id, ip, type, time)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(domain_id, ip) DO UPDATE SET
			time = ?,
			type = ?`,
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

	_, err := db.DomainsDB.Exec(
		`UPDATE domain
		SET resolv_count = resolv_count + 1,
		    last_resolv_time = ?
		WHERE id = ?`,
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

	_, err := db.StatsDB.Exec(
		`INSERT INTO domain_stat (domain, client_ip, rtype, timestamp)
		VALUES (?, ?, ?, ?)`,
		domain, clientIP, rtype, now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert domain stat: %w", err)
	}

	return nil
}
