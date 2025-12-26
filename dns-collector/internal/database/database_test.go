package database

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestInsertOrGetDomain_NewDomain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "domain", "time_insert", "resolv_count", "max_resolv", "last_resolv_time", "last_seen"}).
		AddRow(1, "example.com", now, 0, 10, now, now)

	mock.ExpectQuery(`INSERT INTO domain`).
		WithArgs("example.com", sqlmock.AnyArg(), 10, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	domain, isNew, err := database.InsertOrGetDomain("example.com", 10)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !isNew {
		t.Error("Expected isNew=true for new domain")
	}
	if domain.ID != 1 {
		t.Errorf("Expected ID=1, got %d", domain.ID)
	}
	if domain.Domain != "example.com" {
		t.Errorf("Expected domain=example.com, got %s", domain.Domain)
	}
	if domain.MaxResolv != 10 {
		t.Errorf("Expected MaxResolv=10, got %d", domain.MaxResolv)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestInsertOrGetDomain_ExistingDomain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}
	now := time.Now()

	// First query returns no rows (conflict)
	mock.ExpectQuery(`INSERT INTO domain`).
		WithArgs("example.com", sqlmock.AnyArg(), 10, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)

	// Second query fetches existing domain
	rows := sqlmock.NewRows([]string{"id", "domain", "time_insert", "resolv_count", "max_resolv", "last_resolv_time", "last_seen"}).
		AddRow(1, "example.com", now, 5, 10, now, now)

	mock.ExpectQuery(`SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time, last_seen FROM domain WHERE domain`).
		WithArgs("example.com").
		WillReturnRows(rows)

	domain, isNew, err := database.InsertOrGetDomain("example.com", 10)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if isNew {
		t.Error("Expected isNew=false for existing domain")
	}
	if domain.ID != 1 {
		t.Errorf("Expected ID=1, got %d", domain.ID)
	}
	if domain.ResolvCount != 5 {
		t.Errorf("Expected ResolvCount=5, got %d", domain.ResolvCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestGetDomainsToResolve(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "domain", "time_insert", "resolv_count", "max_resolv", "last_resolv_time", "last_seen"}).
		AddRow(1, "example.com", now, 0, 10, now, now).
		AddRow(2, "test.com", now, 3, 10, now, now)

	mock.ExpectQuery(`SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time, last_seen FROM domain WHERE resolv_count < max_resolv ORDER BY last_resolv_time ASC LIMIT`).
		WithArgs(10).
		WillReturnRows(rows)

	// Test legacy mode (cyclicMode = false)
	domains, err := database.GetDomainsToResolve(10, false, 0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(domains) != 2 {
		t.Fatalf("Expected 2 domains, got %d", len(domains))
	}

	if domains[0].Domain != "example.com" {
		t.Errorf("Expected first domain=example.com, got %s", domains[0].Domain)
	}
	if domains[1].Domain != "test.com" {
		t.Errorf("Expected second domain=test.com, got %s", domains[1].Domain)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestGetDomainsToResolve_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	rows := sqlmock.NewRows([]string{"id", "domain", "time_insert", "resolv_count", "max_resolv", "last_resolv_time", "last_seen"})

	mock.ExpectQuery(`SELECT id, domain, time_insert, resolv_count, max_resolv, last_resolv_time, last_seen FROM domain WHERE resolv_count < max_resolv`).
		WithArgs(10).
		WillReturnRows(rows)

	// Test legacy mode (cyclicMode = false)
	domains, err := database.GetDomainsToResolve(10, false, 0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(domains) != 0 {
		t.Errorf("Expected 0 domains, got %d", len(domains))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestInsertOrUpdateIP_New(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	mock.ExpectExec(`INSERT INTO ip`).
		WithArgs(1, "192.168.1.1", "A", sqlmock.AnyArg(), sqlmock.AnyArg(), "A").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = database.InsertOrUpdateIP(1, "192.168.1.1", "A")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestUpdateDomainResolvStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	mock.ExpectExec(`UPDATE domain SET resolv_count = resolv_count \+ 1, last_resolv_time`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Test legacy mode (cyclicMode = false)
	err = database.UpdateDomainResolvStats(1, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestInsertDomainStat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	mock.ExpectExec(`INSERT INTO domain_stat`).
		WithArgs("example.com", "192.168.1.1", "A", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = database.InsertDomainStat("example.com", "192.168.1.1", "A")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestDeleteOldStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	mock.ExpectExec(`DELETE FROM domain_stat WHERE timestamp <`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 42))

	deleted, err := database.DeleteOldStats(30)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if deleted != 42 {
		t.Errorf("Expected 42 deleted records, got %d", deleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestDeleteOldStats_NoneDeleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	mock.ExpectExec(`DELETE FROM domain_stat WHERE timestamp <`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	deleted, err := database.DeleteOldStats(30)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if deleted != 0 {
		t.Errorf("Expected 0 deleted records, got %d", deleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestDeleteOldDomains_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	// Expect transaction begin
	mock.ExpectBegin()

	// Expect IP deletion
	mock.ExpectExec(`DELETE FROM ip WHERE domain_id IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 5))

	// Expect domain deletion
	mock.ExpectExec(`DELETE FROM domain WHERE last_seen IS NOT NULL AND last_seen <`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect transaction commit
	mock.ExpectCommit()

	domainsDeleted, ipsDeleted, err := database.DeleteOldDomains(30)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if domainsDeleted != 2 {
		t.Errorf("Expected 2 domains deleted, got %d", domainsDeleted)
	}

	if ipsDeleted != 5 {
		t.Errorf("Expected 5 IPs deleted, got %d", ipsDeleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestDeleteOldDomains_NoneDeleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM ip WHERE domain_id IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`DELETE FROM domain WHERE last_seen IS NOT NULL AND last_seen <`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	domainsDeleted, ipsDeleted, err := database.DeleteOldDomains(30)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if domainsDeleted != 0 {
		t.Errorf("Expected 0 domains deleted, got %d", domainsDeleted)
	}

	if ipsDeleted != 0 {
		t.Errorf("Expected 0 IPs deleted, got %d", ipsDeleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestDeleteOldDomains_Disabled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	// Should not execute any queries when TTL is 0 or negative
	domainsDeleted, ipsDeleted, err := database.DeleteOldDomains(0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if domainsDeleted != 0 {
		t.Errorf("Expected 0 domains deleted, got %d", domainsDeleted)
	}

	if ipsDeleted != 0 {
		t.Errorf("Expected 0 IPs deleted, got %d", ipsDeleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestDeleteOldDomains_TransactionError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() { _ = db.Close() }()

	database := &Database{DB: db}

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM ip WHERE domain_id IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	domainsDeleted, ipsDeleted, err := database.DeleteOldDomains(30)
	if err == nil {
		t.Error("Expected error but got nil")
	}

	if domainsDeleted != 0 {
		t.Errorf("Expected 0 domains deleted on error, got %d", domainsDeleted)
	}

	if ipsDeleted != 0 {
		t.Errorf("Expected 0 IPs deleted on error, got %d", ipsDeleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}


