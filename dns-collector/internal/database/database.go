package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
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
	LastSeen       *time.Time // When domain was last queried by client (can be NULL)
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
		last_resolv_time TIMESTAMP NOT NULL,
		last_seen TIMESTAMP
	) WITH (fillfactor = 70);
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
	) WITH (fillfactor = 70);
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

// InsertOrGetDomain inserts a new domain or returns existing one.
// Returns the domain, a boolean indicating if it was newly created, and any error.
func (db *Database) InsertOrGetDomain(domain string, maxResolv int) (*Domain, bool, error) {
	now := time.Now()

	// Use INSERT ... ON CONFLICT for upsert
	var d Domain
	err := db.DB.QueryRow(
		`INSERT INTO domain (domain, time_insert, resolv_count, max_resolv, last_resolv_time, last_seen)
		VALUES ($1, $2, 0, $3, $4, $5)
		ON CONFLICT (domain) DO NOTHING
		RETURNING id, domain, time_insert, resolv_count, max_resolv, last_resolv_time, last_seen`,
		domain, now, maxResolv, now, now,
	).Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime, &d.LastSeen)

	if err == sql.ErrNoRows {
		// Domain already exists, fetch it
		err = db.DB.QueryRow(
			`SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time, last_seen
			FROM domain WHERE domain = $1`,
			domain,
		).Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime, &d.LastSeen)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get existing domain: %w", err)
		}
		return &d, false, nil // existing domain
	} else if err != nil {
		return nil, false, fmt.Errorf("failed to insert domain: %w", err)
	}

	return &d, true, nil // new domain
}

// GetDomainsToResolve returns domains that need to be resolved
// In cyclic mode, includes domains where resolv_count < max_resolv.
// Note: With current cyclic reset logic (reset to 2/3), domains never reach max_resolv,
// making the cooldown condition unreachable. The cooldown branch is preserved for compatibility.
func (db *Database) GetDomainsToResolve(limit int, cyclicMode bool, cooldownMins int) ([]Domain, error) {
	var query string
	var args []interface{}

	if cyclicMode {
		// Cyclic mode: include domains that:
		// 1. Still in current cycle (resolv_count < max_resolv), OR
		// 2. Completed a cycle AND cooldown period has passed
		cooldownTime := time.Now().Add(-time.Duration(cooldownMins) * time.Minute)
		query = `SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time, last_seen
			FROM domain
			WHERE resolv_count < max_resolv
			   OR (resolv_count >= max_resolv AND last_resolv_time < $1)
			ORDER BY last_resolv_time ASC
			LIMIT $2`
		args = []interface{}{cooldownTime, limit}
	} else {
		// Legacy mode: only domains that haven't reached max_resolv
		query = `SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time, last_seen
			FROM domain
			WHERE resolv_count < max_resolv
			ORDER BY last_resolv_time ASC
			LIMIT $1`
		args = []interface{}{limit}
	}

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Domain, &d.TimeInsert, &d.ResolvCount, &d.MaxResolv, &d.LastResolvTime, &d.LastSeen); err != nil {
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

// InsertDomainStatsBatch inserts a batch of statistics rows in one statement.
func (db *Database) InsertDomainStatsBatch(stats []DomainStat) error {
	if len(stats) == 0 {
		return nil
	}

	domains := make([]string, len(stats))
	clientIPs := make([]string, len(stats))
	rtypes := make([]string, len(stats))
	timestamps := make([]string, len(stats))
	for i, s := range stats {
		domains[i] = s.Domain
		clientIPs[i] = s.ClientIP
		rtypes[i] = s.RType
		timestamps[i] = formatTimestamp(s.Timestamp)
	}

	_, err := db.DB.Exec(
		`INSERT INTO domain_stat (domain, client_ip, rtype, timestamp)
		 SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::timestamp[])`,
		pq.Array(domains), pq.Array(clientIPs), pq.Array(rtypes), pq.Array(timestamps),
	)
	if err != nil {
		return fmt.Errorf("failed to insert domain stats batch: %w", err)
	}
	return nil
}

// UpdateLastSeenBatch updates last_seen for many domains in one statement.
// Rows updated less than minInterval ago are left untouched to avoid churn.
func (db *Database) UpdateLastSeenBatch(updates map[int64]time.Time, minInterval time.Duration) (int64, error) {
	if len(updates) == 0 {
		return 0, nil
	}
	if minInterval < 0 {
		minInterval = 0
	}

	ids := make([]int64, 0, len(updates))
	timestamps := make([]string, 0, len(updates))
	for id, ts := range updates {
		ids = append(ids, id)
		timestamps = append(timestamps, formatTimestamp(ts))
	}

	res, err := db.DB.Exec(
		`UPDATE domain d
		 SET last_seen = v.ts
		 FROM (SELECT * FROM unnest($1::bigint[], $2::timestamp[]) AS v(id, ts)) v
		 WHERE d.id = v.id
		   AND (d.last_seen IS NULL OR d.last_seen < v.ts - make_interval(secs => $3))`,
		pq.Array(ids), pq.Array(timestamps), int(minInterval.Seconds()),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to update last_seen batch: %w", err)
	}

	updated, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get last_seen rows affected: %w", err)
	}
	return updated, nil
}

// UpsertIPs inserts new addresses and refreshes timestamps for returned addresses.
// Addresses that are no longer returned are deliberately left untouched: they age out
// through the normal IP retention cleanup instead of causing delete/insert churn on
// DNS round-robin answers.
func (db *Database) UpsertIPs(domainID int64, ipType string, ips []string, refresh time.Duration) error {
	if len(ips) == 0 {
		return nil
	}
	if refresh <= 0 {
		refresh = 24 * time.Hour
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin IP upsert: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Error rolling back IP upsert: %v", err)
		}
	}()

	now := time.Now()
	if _, err := tx.Exec(
		`INSERT INTO ip (domain_id, ip, type, time)
		 SELECT $1, unnest($2::text[]), $3, $4::timestamp
		 ON CONFLICT (domain_id, ip) DO UPDATE
		 SET time = EXCLUDED.time
		 WHERE ip.time < EXCLUDED.time - make_interval(secs => $5)`,
		domainID, pq.Array(ips), ipType, now, int(refresh.Seconds()),
	); err != nil {
		return fmt.Errorf("failed to upsert IPs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit IP upsert: %w", err)
	}
	return nil
}

func formatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.999999")
}

// UpdateDomainResolvStats updates resolv_count and last_resolv_time
// In cyclic mode, resets resolv_count to ⌊max_resolv × 2/3⌋ when it reaches max_resolv - 1
// This prevents domains from completing a full cycle and triggering cooldown logic
func (db *Database) UpdateDomainResolvStats(domainID int64, cyclicMode bool) error {
	now := time.Now()

	var query string
	if cyclicMode {
		// Reset to ⌊max_resolv × 2/3⌋ when reaching max_resolv - 1
		// This keeps domains in a partial cycle rather than full reset to 0
		// Examples: max_resolv=10 → reset to 6, max_resolv=3 → reset to 2
		query = `UPDATE domain
			SET resolv_count = CASE
				WHEN resolv_count >= max_resolv - 1 THEN CAST(max_resolv * 2.0 / 3.0 AS INTEGER)
				ELSE resolv_count + 1
			END,
			last_resolv_time = $1
			WHERE id = $2`
	} else {
		// Legacy mode: just increment
		query = `UPDATE domain
			SET resolv_count = resolv_count + 1,
			    last_resolv_time = $1
			WHERE id = $2`
	}

	_, err := db.DB.Exec(query, now, domainID)
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

// DeleteOldStats deletes statistics records older than the specified number of days.
// For a partitioned domain_stat it first drops whole old monthly partitions,
// then deletes the remaining tail rows from the oldest retained partition.
func (db *Database) DeleteOldStats(retentionDays int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	partitioned, err := db.isDomainStatPartitioned()
	if err != nil {
		return 0, fmt.Errorf("failed to inspect domain_stat partitioning: %w", err)
	}
	if !partitioned {
		return db.deleteOldStatsRows(cutoffTime)
	}

	dropped, err := db.dropOldDomainStatPartitions(cutoffTime)
	if err != nil {
		return 0, err
	}
	deleted, err := db.deleteOldStatsRows(cutoffTime)
	if err != nil {
		return 0, err
	}
	return dropped + deleted, nil
}

func (db *Database) deleteOldStatsRows(cutoffTime time.Time) (int64, error) {
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

func (db *Database) isDomainStatPartitioned() (bool, error) {
	var partitioned bool
	err := db.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_partitioned_table pt
			JOIN pg_class c ON c.oid = pt.partrelid
			WHERE c.relname = 'domain_stat'
			  AND c.relnamespace = 'public'::regnamespace
		)`).Scan(&partitioned)
	if err != nil {
		return false, err
	}
	return partitioned, nil
}

// EnsureDomainStatPartitions creates the current and next months' partitions.
// It is a no-op for a non-partitioned table (old schema).
func (db *Database) EnsureDomainStatPartitions(now time.Time, monthsAhead int) error {
	partitioned, err := db.isDomainStatPartitioned()
	if err != nil {
		return err
	}
	if !partitioned {
		return nil
	}
	if monthsAhead < 0 {
		monthsAhead = 0
	}

	for i := 0; i <= monthsAhead; i++ {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0)
		end := start.AddDate(0, 1, 0)
		name := "domain_stat_" + start.Format("2006_01")
		ddl := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s PARTITION OF domain_stat FOR VALUES FROM ('%s') TO ('%s')`,
			pq.QuoteIdentifier(name), start.Format("2006-01-02"), end.Format("2006-01-02"),
		)
		if _, err := db.DB.Exec(ddl); err != nil {
			return fmt.Errorf("failed to create partition %s: %w", name, err)
		}
	}
	return nil
}

func (db *Database) dropOldDomainStatPartitions(cutoffTime time.Time) (int64, error) {
	rows, err := db.DB.Query(`
		SELECT child.relname,
		       left(split_part(pg_get_expr(child.relpartbound, child.oid), '''', 4), 10)
		FROM pg_inherits
		JOIN pg_class parent ON parent.oid = inhparent
		JOIN pg_class child ON child.oid = inhrelid
		WHERE parent.relname = 'domain_stat'
		  AND parent.relnamespace = 'public'::regnamespace`)
	if err != nil {
		return 0, fmt.Errorf("failed to list domain_stat partitions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type partition struct {
		name  string
		upper time.Time
	}
	var old []partition
	for rows.Next() {
		var name string
		var upper sql.NullString
		if err := rows.Scan(&name, &upper); err != nil {
			return 0, fmt.Errorf("failed to scan domain_stat partition: %w", err)
		}
		if !upper.Valid {
			continue
		}
		t, err := time.Parse("2006-01-02", upper.String)
		if err != nil {
			continue
		}
		if !t.After(cutoffTime) {
			old = append(old, partition{name: name, upper: t})
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("failed to read domain_stat partitions: %w", err)
	}
	_ = rows.Close()

	var dropped int64
	for _, p := range old {
		var n int64
		if err := db.DB.QueryRow(fmt.Sprintf(`SELECT count(*) FROM %s`, pq.QuoteIdentifier(p.name))).Scan(&n); err != nil {
			return dropped, fmt.Errorf("failed to count rows in %s: %w", p.name, err)
		}
		if _, err := db.DB.Exec(fmt.Sprintf(`DROP TABLE %s`, pq.QuoteIdentifier(p.name))); err != nil {
			return dropped, fmt.Errorf("failed to drop partition %s: %w", p.name, err)
		}
		dropped += n
	}
	return dropped, nil
}

// UpdateDomainLastSeen updates the last_seen timestamp for a domain
// Called when a DNS query is received for the domain
func (db *Database) UpdateDomainLastSeen(domainID int64) error {
	now := time.Now()

	_, err := db.DB.Exec(
		`UPDATE domain SET last_seen = $1 WHERE id = $2`,
		now, domainID,
	)
	if err != nil {
		return fmt.Errorf("failed to update domain last_seen: %w", err)
	}

	return nil
}

// GetDomainsCount returns the total number of domains in the database.
func (db *Database) GetDomainsCount() (int64, error) {
	var count int64
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM domain`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count domains: %w", err)
	}
	return count, nil
}

// GetIPsCount returns the total number of IP addresses in the database.
func (db *Database) GetIPsCount() (int64, error) {
	var count int64
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM ip`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count IPs: %w", err)
	}
	return count, nil
}

// DeleteExpiredIPs deletes IP addresses older than the specified TTL
// Only deletes IPs for domains that are still being queried (last_seen >= cutoff)
// IPs of inactive domains are preserved
func (db *Database) DeleteExpiredIPs(ttlDays int) (int64, error) {
	if ttlDays <= 0 {
		return 0, nil // TTL disabled
	}

	cutoffTime := time.Now().AddDate(0, 0, -ttlDays)

	// Delete old IPs only for active domains (domains that have been queried recently)
	// This protects IPs of domains that are no longer being queried
	result, err := db.DB.Exec(
		`DELETE FROM ip
		WHERE time < $1
		AND domain_id IN (
			SELECT id FROM domain
			WHERE last_seen >= $1
		)`,
		cutoffTime,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired IPs: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return deleted, nil
}

// DeleteOldDomains deletes domains not seen in the specified TTL period
// Also explicitly deletes associated IPs first for better metrics tracking
// Domains with NULL last_seen are preserved (never queried, only resolved)
// Returns counts: (domains deleted, IPs deleted, error)
func (db *Database) DeleteOldDomains(ttlDays int) (int64, int64, error) {
	if ttlDays <= 0 {
		return 0, 0, nil // TTL disabled
	}

	cutoffTime := time.Now().AddDate(0, 0, -ttlDays)

	// Start transaction for atomic operation
	tx, err := db.DB.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Step 1: Explicitly delete IPs for old domains (for metrics tracking)
	// Only delete domains where last_seen is NOT NULL and is old
	ipResult, err := tx.Exec(
		`DELETE FROM ip
		WHERE domain_id IN (
			SELECT id FROM domain
			WHERE last_seen IS NOT NULL
			AND last_seen < $1
		)`,
		cutoffTime,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to delete IPs for old domains: %w", err)
	}

	ipsDeleted, err := ipResult.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get IP rows affected: %w", err)
	}

	// Step 2: Delete old domains (with non-NULL last_seen)
	// Domains with NULL last_seen are preserved (these are domains that were
	// added for resolution but never actually queried by clients)
	domainResult, err := tx.Exec(
		`DELETE FROM domain
		WHERE last_seen IS NOT NULL
		AND last_seen < $1`,
		cutoffTime,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to delete old domains: %w", err)
	}

	domainsDeleted, err := domainResult.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get domain rows affected: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return domainsDeleted, ipsDeleted, nil
}
